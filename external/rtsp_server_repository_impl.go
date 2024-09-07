package external

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type ServerAttributes struct {
	RTSPAddress       string `json:"rtsp_address"`
	UDPRTPAddress     string `json:"udp_rtp_address"`
	UDPRTCPAddress    string `json:"udp_rtcp_address"`
	MulticastIPRange  string `json:"multicast_ip_range"`
	MulticastRTPPort  int    `json:"multicast_rtp_port"`
	MulticastRTCPPort int    `json:"multicast_rtcp_port"`
}

func (sa *ServerAttributes) SetupRtspServer() (*http.Response, error) {
	externalRtspServer := os.Getenv("EXTERNAL_SETUP_SERVER_URL")
	serverParametersBody, err := json.Marshal(sa)
	if err != nil {
		return &http.Response{}, err
	}
	proxyReq, err := http.NewRequest("POST", externalRtspServer, bytes.NewReader(serverParametersBody))
	if err != nil {
		return nil, err
	}
	httpClient := http.Client{}
	resp, err := httpClient.Do(proxyReq)
	if err != nil {
		return &http.Response{}, err
	}

	return resp, nil
}
