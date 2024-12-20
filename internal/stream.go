package internal

import (
	"context"
	"fmt"
	"log/slog"
	"time"
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

func (service *StreamerService) createVideoStream(videoName string) chan string {
	portChan := make(chan int)
	errChan := make(chan error)

	go func() {
		rtspServer, err := rtspserver.ConfigureRtspServer(portChan, service.Context, service.Logger)
		if err != nil {
			service.Logger.Error("error while setuping RTSP server", "err", err)
			errChan <- err
			return
		}

		if err := rtspServer.StartAndWait(); err != nil {
			service.Logger.Error("error while starting RTSP server", "err", err)
			service.CtxCancel()
		}
	}()

	rtspUrlChan := make(chan string)

	time.Sleep(time.Second * 1)

	go func() {
		select {
		case port := <-portChan:
			close(portChan)
			close(errChan)
			rtspUrl := fmt.Sprintf("%s:%d", service.Envs.RtspStreamUrlPattern, port)
			service.Logger.Debug("RTSP server configured and running", "RTSP_URL", rtspUrl)

			rtspUrlChan <- rtspUrl

			if err := service.VideoService.streamVideoToServer(videoName, rtspUrl); err != nil {
				service.CtxCancel()
			}
		case err := <-errChan:
			close(errChan)
			close(rtspUrlChan)
			service.Logger.Error("couldn't setup RTSP-server => stream wasn't started", "err", err)
		}
	}()

	return rtspUrlChan
}
