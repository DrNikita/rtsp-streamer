package internal

import "github.com/pion/webrtc/v4"

type Response struct {
	Status       int
	IsConverting bool
	Result       any
	Error        string
}

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type peerConnectionState struct {
	peerConnection *webrtc.PeerConnection
	websocket      *threadSafeWriter
}
