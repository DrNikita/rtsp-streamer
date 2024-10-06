package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"video-handler/configs"
	"video-handler/internal/rtspserver"
)

type StreamerService struct {
	VideoService *VideoService
	Envs         *configs.EnvVariables
	Logger       *slog.Logger
	Context      context.Context
	CtxCancel    context.CancelFunc
}

func NewStreamerService(service *VideoService, envs *configs.EnvVariables, logger *slog.Logger, ctx context.Context, ctxCancel context.CancelFunc) *StreamerService {
	return &StreamerService{
		VideoService: service,
		Envs:         envs,
		Logger:       logger,
		Context:      ctx,
		CtxCancel:    ctxCancel,
	}
}

func (service *StreamerService) createVideoStream(videoName string) (string, error) {
	freePort, err := findFreePort()
	if err != nil {
		return "", err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wg.Done()
		rtspServer := rtspserver.ConfigureRtspServer(":"+strconv.Itoa(freePort), service.Context)
		err := rtspServer.StartAndWait()
		if err != nil {
			service.CtxCancel()
		}
	}()

	wg.Wait()

	rtspUrl := fmt.Sprintf("%s:%d", service.Envs.RtspStreamUrlPattern, freePort)
	service.Logger.Debug("RTSP server configured and running", "RTSP_URL", rtspUrl)

	wg.Add(1)
	go func() {
		wg.Done()
		err = service.VideoService.streamVideoToServer(videoName, rtspUrl)
		if err != nil {
			service.CtxCancel()
		}
	}()

	wg.Wait()

	// time.Sleep(1 * time.Hour)

	return rtspUrl, nil
}

func findFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}
