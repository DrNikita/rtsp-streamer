package webrtc

import (
	"net/http"

	"github.com/pion/webrtc/v3"
)

type WebrtcInterface interface {
	addTrack(t *webrtc.TrackLocalStaticSample) error
	removeTrack(t *webrtc.TrackLocalStaticSample)
	signalPeerConnections()
	dispatchKeyFrame()
	websocketHandler(w http.ResponseWriter, r *http.Request)
}
