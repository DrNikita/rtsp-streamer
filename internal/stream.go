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
	Service   *VideoService
	Envs      *configs.EnvVariables
	Logger    *slog.Logger
	Context   context.Context
	CtxCancel context.CancelFunc
}

func NewStreamerService(service *VideoService, envs *configs.EnvVariables, logger *slog.Logger, ctx context.Context, ctxCancel context.CancelFunc) *StreamerService {
	return &StreamerService{
		Service:   service,
		Envs:      envs,
		Logger:    logger,
		Context:   ctx,
		CtxCancel: ctxCancel,
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

	wg.Add(1)
	go func() {
		wg.Done()
		err = service.Service.streamVideoToServer(videoName, rtspUrl)
		if err != nil {
			service.CtxCancel()
		}
	}()

	wg.Wait()

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
