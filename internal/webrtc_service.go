// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// sfu-ws is a many-to-many websocket based SFU
package internal

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

var i int

type WebrtcManager interface {
	addTrack(t *webrtc.TrackLocalStaticSample) error
	removeTrack(t *webrtc.TrackLocalStaticSample)
	signalPeerConnections()
	dispatchKeyFrame()
	websocketHandler(w http.ResponseWriter, r *http.Request)
}

type WebrtcRepository struct {
	upgrader        websocket.Upgrader
	listLock        sync.RWMutex
	indexTemplate   *template.Template
	peerConnections []peerConnectionState
	trackLocals     map[string]*webrtc.TrackLocalStaticRTP

	streamerService *StreamerService

	logger *slog.Logger
}

func NewWebrtcRepository(streamerService *StreamerService, logger *slog.Logger) *WebrtcRepository {
	indexHTML, err := os.ReadFile("./static/index.html")
	if err != nil {
		panic(err)
	}
	indexTemplate := template.Must(template.New("").Parse(string(indexHTML)))

	return &WebrtcRepository{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		indexTemplate:   indexTemplate,
		listLock:        sync.RWMutex{},
		peerConnections: make([]peerConnectionState, 0),
		trackLocals:     map[string]*webrtc.TrackLocalStaticRTP{},

		streamerService: streamerService,

		logger: logger,
	}
}

// func (wr *WebrtcRepository) RegisterRoutes(r chi.Router) {
// 	r.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
// 		var response struct {
// 			Error   string `json:"error,omitempty"`
// 			Success string `json:"success,omitempty"`
// 		}

// 		var requestBody struct {
// 			VideoSource string `json:"rtsp_url"`
// 		}
// 		bodyBytes, err := io.ReadAll(r.Body)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		}

// 		err = json.Unmarshal(bodyBytes, &requestBody)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		}

// 		w.Header().Set("Content-Type", "application/json")

// 		err = wr.publishNewStream(requestBody.VideoSource)
// 		if err != nil {
// 			http.Error(w, err.Error(), http.StatusBadRequest)
// 		}

// 		response.Success = "success"
// 		json.NewEncoder(w).Encode(response)
// 	})
// }

func (wr *WebrtcRepository) InitConnection(r chi.Router) {
	r.HandleFunc("/websocket", wr.websocketHandler)

	// index.html handler
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := wr.indexTemplate.Execute(w, "ws://"+r.Host+"/websocket"); err != nil {
			log.Fatal(err)
		}
	})

	// request a keyframe every 3 seconds
	go func() {
		for range time.NewTicker(time.Microsecond * 500).C {
			wr.dispatchKeyFrame()
		}
	}()
}

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type peerConnectionState struct {
	peerConnection *webrtc.PeerConnection
	websocket      *threadSafeWriter
}

// Add to list of tracks and fire renegotation for all PeerConnections
func (wr *WebrtcRepository) addTrack(t *webrtc.TrackLocalStaticRTP) error {
	wr.listLock.Lock()
	defer func() {
		wr.listLock.Unlock()
		wr.signalPeerConnections()
	}()

	wr.trackLocals[t.ID()] = t
	return nil
}

// Remove from list of tracks and fire renegotation for all PeerConnections
func (wr *WebrtcRepository) removeTrack(t *webrtc.TrackLocalStaticRTP) {
	wr.listLock.Lock()
	defer func() {
		wr.listLock.Unlock()
		wr.signalPeerConnections()
	}()

	delete(wr.trackLocals, t.ID())
}

// signalPeerConnections updates each PeerConnection so that it is getting all the expected media tracks
func (wr *WebrtcRepository) signalPeerConnections() {
	wr.listLock.Lock()
	defer func() {
		wr.listLock.Unlock()
		wr.dispatchKeyFrame()
	}()

	attemptSync := func() (tryAgain bool) {
		for i := range wr.peerConnections {
			if wr.peerConnections[i].peerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
				wr.peerConnections = append(wr.peerConnections[:i], wr.peerConnections[i+1:]...)
				return true // We modified the slice, start from the beginning
			}

			// map of sender we already are seanding, so we don't double send
			existingSenders := map[string]bool{}

			for _, sender := range wr.peerConnections[i].peerConnection.GetSenders() {
				if sender.Track() == nil {
					continue
				}

				existingSenders[sender.Track().ID()] = true

				// If we have a RTPSender that doesn't map to a existing track remove and signal
				if _, ok := wr.trackLocals[sender.Track().ID()]; !ok {
					if err := wr.peerConnections[i].peerConnection.RemoveTrack(sender); err != nil {
						return true
					}
				}
			}

			// Don't receive videos we are sending, make sure we don't have loopback
			for _, receiver := range wr.peerConnections[i].peerConnection.GetReceivers() {
				if receiver.Track() == nil {
					continue
				}

				existingSenders[receiver.Track().ID()] = true
			}

			// Add all track we aren't sending yet to the PeerConnection
			for trackID := range wr.trackLocals {
				if _, ok := existingSenders[trackID]; !ok {

					if _, err := wr.peerConnections[i].peerConnection.AddTransceiverFromTrack(wr.trackLocals[trackID], webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
						return true
					}
				}
			}

			offer, err := wr.peerConnections[i].peerConnection.CreateOffer(nil)
			if err != nil {
				return true
			}

			if err = wr.peerConnections[i].peerConnection.SetLocalDescription(offer); err != nil {
				return true
			}

			offerString, err := json.Marshal(offer)
			if err != nil {
				return true
			}

			if err = wr.peerConnections[i].websocket.WriteJSON(&websocketMessage{
				Event: "offer",
				Data:  string(offerString),
			}); err != nil {
				return true
			}
		}

		return
	}

	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			// Release the lock and attempt a sync in 3 seconds. We might be blocking a RemoveTrack or AddTrack
			go func() {
				time.Sleep(time.Second * 3)
				wr.signalPeerConnections()
			}()
			return
		}

		if !attemptSync() {
			break
		}
	}
}

// dispatchKeyFrame sends a keyframe to all PeerConnections, used everytime a new user joins the call
func (wr *WebrtcRepository) dispatchKeyFrame() {
	wr.listLock.Lock()
	defer wr.listLock.Unlock()

	for i := range wr.peerConnections {
		for _, receiver := range wr.peerConnections[i].peerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = wr.peerConnections[i].peerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()),
				},
			})
		}
	}
}

// Handle incoming websockets
func (wr *WebrtcRepository) websocketHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP request to Websocket
	unsafeConn, err := wr.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}

	c := &threadSafeWriter{unsafeConn, sync.Mutex{}}

	// When this frame returns close the Websocket
	defer c.Close() //nolint

	// Create new PeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Print(err)
		return
	}

	// When this frame returns close the PeerConnection
	defer peerConnection.Close() //nolint

	// Add our new PeerConnection to global list
	wr.listLock.Lock()
	wr.peerConnections = append(wr.peerConnections, peerConnectionState{peerConnection, c})
	wr.listLock.Unlock()

	// Trickle ICE. Emit server candidate to client
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}

		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Println(err)
			return
		}

		if writeErr := c.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Println(writeErr)
		}
	})

	// If PeerConnection is closed remove it from global list
	peerConnection.OnConnectionStateChange(func(p webrtc.PeerConnectionState) {
		switch p {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				log.Print(err)
			}
		case webrtc.PeerConnectionStateClosed:
			wr.signalPeerConnections()
		default:
		}
	})

	_, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly})
	if err != nil {
		panic(err)
	}

	processRTCP := func(rtpSender *webrtc.RTPSender) {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}
	for _, rtpSender := range peerConnection.GetSenders() {
		go processRTCP(rtpSender)
	}

	// Signal for the new PeerConnection
	wr.signalPeerConnections()

	message := &websocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		} else if err := json.Unmarshal(raw, &message); err != nil {
			log.Println(err)
			return
		}

		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Println(err)
				return
			}

			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Println(err)
				return
			}
		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				wr.logger.Error("", "err", err.Error())
				return
			}

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				wr.logger.Error("", "err", err.Error())
				return
			}
		case "publish":
			videoName := strings.Replace(message.Data, "\"", "", -1)
			wr.logger.Debug("video name received", "data", videoName)

			rtspUrl, err := wr.streamerService.createVideoStream(videoName)
			if err != nil {
				wr.logger.Error("", "err", err.Error())
				return
			}

			time.Sleep(1 * time.Second)

			wr.publishNewStream(rtspUrl)
		}
	}
}

// Helper to make Gorilla Websockets threadsafe
type threadSafeWriter struct {
	*websocket.Conn
	sync.Mutex
}

func (t *threadSafeWriter) WriteJSON(v interface{}) error {
	t.Lock()
	defer t.Unlock()

	return t.Conn.WriteJSON(v)
}

func (wr *WebrtcRepository) publishNewStream(rtspUrl string) error {
	i++
	rtpTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "pion-"+strconv.Itoa(i), "pion-"+strconv.Itoa(i))
	if err != nil {
		log.Fatal(err)
	}

	err = wr.addTrack(rtpTrack)
	if err != nil {
		return err
	}

	go wr.rtspConsumer(rtpTrack, rtspUrl)

	return nil
}

func (wr *WebrtcRepository) rtspConsumer(track *webrtc.TrackLocalStaticRTP, rtspUrl string) {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL(rtspUrl)
	if err != nil {
		wr.logger.Error("failed to parse url", "RTSP_URL", rtspUrl, "err", err.Error())
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		wr.logger.Error(err.Error())
		panic(err)
	}
	defer c.Close()

	// find available medias
	desc, _, err := c.Describe(u)
	if err != nil {
		wr.logger.Error("failed to describe url", "RTSP_URL", rtspUrl, "err", err.Error())
		panic(err)
	}

	// setup all medias
	err = c.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		wr.logger.Error(err.Error())
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		track.WriteRTP(pkt)
	})

	// called when a RTCP packet arrives
	c.OnPacketRTCPAny(func(medi *description.Media, pkt rtcp.Packet) {
		log.Printf("RTCP packet from media %v, type %T\n", medi, pkt)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		wr.logger.Error(err.Error())
		panic(err)
	}

	// wait until a fatal error
	err = c.Wait()
	if err != nil {
		wr.logger.Error(err.Error())
	}
}
