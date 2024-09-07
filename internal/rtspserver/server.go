package rtspserver

import (
	"context"
	"log"
	"sync"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
)

// This example shows how to
// 1. create a RTSP server which accepts plain connections
// 2. allow a single client to publish a stream with TCP or UDP
// 3. allow multiple clients to read that stream with TCP, UDP or UDP-multicast

type serverHandler struct {
	s         *gortsplib.Server
	mutex     sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
	ctx       context.Context
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("conn opened")
	}
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("conn closed (%v)", ctx.Error)
	}
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("session opened")
	}
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("session closed")
	}

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if the session is the publisher,
	// close the stream and disconnect any reader.
	if sh.stream != nil && ctx.Session == sh.publisher {
		sh.stream.Close()
		sh.stream = nil
	}
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("describe request")
	}

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	// send medias that are being published to the client
	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("announce request")
	}

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// disconnect existing publisher
	if sh.stream != nil {
		sh.stream.Close()
		sh.publisher.Close()
	}

	// create the stream and save the publisher
	sh.stream = gortsplib.NewServerStream(sh.s, ctx.Description)
	sh.publisher = ctx.Session

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("setup request")
	}

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("play request")
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	select {
	case <-sh.ctx.Done():
		sh.s.Close()
	default:
		log.Printf("record request")
	}

	// called when receiving a RTP packet
	ctx.Session.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		// route the RTP packet to all readers
		sh.stream.WritePacketRTP(medi, pkt)
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func ConfigureServer(
	rtspAddress,
	udpRtpAddress,
	udpRtcpAddress,
	multicastIpRange string,
	multicastRtpPort,
	multicastRtcpPort int,
	ctx context.Context,
) *gortsplib.Server {

	h := &serverHandler{
		ctx: ctx,
	}
	h.s = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       rtspAddress,
		UDPRTPAddress:     udpRtpAddress,
		UDPRTCPAddress:    udpRtcpAddress,
		MulticastIPRange:  multicastIpRange,
		MulticastRTPPort:  multicastRtpPort,
		MulticastRTCPPort: multicastRtcpPort,
	}

	log.Printf("RTSP server is ready and running on port: " + rtspAddress)

	return h.s
}

func ConfigureRtspServer(rtspAddress string, ctx context.Context) *gortsplib.Server {
	h := &serverHandler{
		ctx: ctx,
	}
	h.s = &gortsplib.Server{
		Handler:     h,
		RTSPAddress: rtspAddress,
	}

	log.Printf("RTSP server is ready and running on port: " + rtspAddress)

	return h.s
}
