// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

// sfu-ws is a many-to-many websocket based SFU
package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/deepch/vdk/av"
	"github.com/deepch/vdk/codec/h264parser"
	"github.com/deepch/vdk/format/rtsp"
	"github.com/deepch/vdk/format/rtspv2"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
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
	trackLocals     map[string]*webrtc.TrackLocalStaticSample
}

func NewWebrtcRepository() *WebrtcRepository {
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
		trackLocals:     map[string]*webrtc.TrackLocalStaticSample{},
	}
}

func (wr *WebrtcRepository) RegisterRoutes(r chi.Router) {
	r.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		var response struct {
			Error   string `json:"error,omitempty"`
			Success string `json:"success,omitempty"`
		}

		var requestBody struct {
			VideoSource string `json:"rtsp_url"`
		}
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		err = json.Unmarshal(bodyBytes, &requestBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		w.Header().Set("Content-Type", "application/json")

		err = wr.publishNewStream(requestBody.VideoSource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		response.Success = "success"
		json.NewEncoder(w).Encode(response)
	})
}

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
func (wr *WebrtcRepository) addTrack(t *webrtc.TrackLocalStaticSample) error {
	wr.listLock.Lock()
	defer func() {
		wr.listLock.Unlock()
		wr.signalPeerConnections()
	}()

	wr.trackLocals[t.ID()] = t
	return nil
}

// Remove from list of tracks and fire renegotation for all PeerConnections
func (wr *WebrtcRepository) removeTrack(t *webrtc.TrackLocalStaticSample) {
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
	//////////////////////////////
	// params := transceiver.Sender().GetParameters()
	// for i := range params.Encodings {
	// 	// params.Encodings[i].RID = webrtc.string(StatsTypeReceiver) (5000000) // 5 Mbps
	// 	params.Encodings[i].SSRC = 1.0 // Оставить разрешение как есть
	// }

	// err = transceiver.Sender().AddEncoding(transceiver.Sender().Track())
	// if err != nil {
	// 	log.Fatal(err)
	// }

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: "video/h264",
	}, "pion-rtsp", "pion-rtsp")
	if err != nil {
		log.Fatal(err)
	}

	err = wr.addTrack(videoTrack)
	if err != nil {
		log.Fatal(err)
	}
	defer wr.removeTrack(videoTrack)

	go rtspConsumerV2(videoTrack, "rtsp://localhost:8554")

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
				log.Println(err)
				return
			}

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Println(err)
				return
			}
		case "publish":
			i++
			videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
				MimeType: "video/h264",
			}, "pion-"+strconv.Itoa(i), "pion-"+strconv.Itoa(i))
			if err != nil {
				log.Fatal(err)
			}

			rtspUrl := strings.Replace(message.Data, "\"", "", -1)
			fmt.Println(rtspUrl)

			err = wr.addTrack(videoTrack)
			if err != nil {
				log.Fatal(err)
			}
			defer wr.removeTrack(videoTrack)

			go rtspConsumerV2(videoTrack, rtspUrl)
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
	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: "video/h264",
	}, "pion-rtsp", "pion-rtsp")
	if err != nil {
		return err
	}

	err = wr.addTrack(videoTrack)
	if err != nil {
		return err
	}

	go rtspConsumerV2(videoTrack, rtspUrl)

	return nil
}

// Connect to an RTSP URL and pull media.
// Convert H264 to Annex-B, then write to outboundVideoTrack which sends to all PeerConnections
func rtspConsumer(track *webrtc.TrackLocalStaticSample, rtspUrl string) {
	annexbNALUStartCode := func() []byte { return []byte{0x00, 0x00, 0x00, 0x01} }

	for {
		session, err := rtsp.Dial(rtspUrl)
		if err != nil {
			panic(err)
		}
		session.RtpKeepAliveTimeout = 10 * time.Second

		codecs, err := session.Streams()
		if err != nil {
			panic(err)
		}
		for i, t := range codecs {
			log.Println("Stream", i, "is of type", t.Type().String())
		}
		if codecs[0].Type() != av.H264 {
			panic("RTSP feed must begin with a H264 codec")
		}
		if len(codecs) != 1 {
			log.Println("Ignoring all but the first stream.")
		}

		var previousTime time.Duration
		for {
			pkt, err := session.ReadPacket()
			if err != nil {
				break
			}

			if pkt.Idx != 0 {
				//audio or other stream, skip it
				continue
			}

			pkt.Data = pkt.Data[4:]

			// For every key-frame pre-pend the SPS and PPS
			if pkt.IsKeyFrame {
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(codecs[0].(h264parser.CodecData).PPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
				pkt.Data = append(codecs[0].(h264parser.CodecData).SPS(), pkt.Data...)
				pkt.Data = append(annexbNALUStartCode(), pkt.Data...)
			}

			bufferDuration := pkt.Time - previousTime
			previousTime = pkt.Time
			if err = track.WriteSample(media.Sample{Data: pkt.Data, Duration: bufferDuration}); err != nil && err != io.ErrClosedPipe {
				panic(err)
			}
		}

		if err = session.Close(); err != nil {
			log.Println("session Close error", err)
		}

		time.Sleep(1 * time.Second)
	}
}

func rtspConsumerV2(track *webrtc.TrackLocalStaticSample, rtspUrl string) {
	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: rtspUrl, DisableAudio: true, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: true})
	if err != nil {
		panic(err)
	}
	var previousTime time.Duration
	for {
		select {
		case packetAV := <-RTSPClient.OutgoingPacketQueue:
			if packetAV.IsKeyFrame {
				bufferDuration := packetAV.Time - previousTime
				previousTime = packetAV.Time
				if err = track.WriteSample(media.Sample{Data: packetAV.Data, Duration: bufferDuration}); err != nil && err != io.ErrClosedPipe {
					panic(err)
				}
			}
		}
	}
}

// func RTSPWorker(name, url string, OnDemand, DisableAudio, Debug bool) error {
// 	keyTest := time.NewTimer(20 * time.Second)
// 	clientTest := time.NewTimer(20 * time.Second)
// 	//add next TimeOut
// 	RTSPClient, err := rtspv2.Dial(rtspv2.RTSPClientOptions{URL: url, DisableAudio: DisableAudio, DialTimeout: 3 * time.Second, ReadWriteTimeout: 3 * time.Second, Debug: Debug})
// 	if err != nil {
// 		return err
// 	}
// 	defer RTSPClient.Close()
// 	if RTSPClient.CodecData != nil {
// 		Config.coAd(name, RTSPClient.CodecData)
// 	}
// 	var AudioOnly bool
// 	if len(RTSPClient.CodecData) == 1 && RTSPClient.CodecData[0].Type().IsAudio() {
// 		AudioOnly = true
// 	}
// for {
// 	select {
// 	case <-clientTest.C:
// 		if OnDemand {
// 			if !Config.HasViewer(name) {
// 				return ErrorStreamExitNoViewer
// 			} else {
// 				clientTest.Reset(20 * time.Second)
// 			}
// 		}
// 	case <-keyTest.C:
// 		return ErrorStreamExitNoVideoOnStream
// 	case signals := <-RTSPClient.Signals:
// 		switch signals {
// 		case rtspv2.SignalCodecUpdate:
// 			Config.coAd(name, RTSPClient.CodecData)
// 		case rtspv2.SignalStreamRTPStop:
// 			return ErrorStreamExitRtspDisconnect
// 		}
// 	case packetAV := <-RTSPClient.OutgoingPacketQueue:
// 		if AudioOnly || packetAV.IsKeyFrame {
// 			keyTest.Reset(20 * time.Second)
// 		}
// 		Config.cast(name, *packetAV)
// 	}
// }
// }
