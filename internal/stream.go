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
	// Каналы для общения между горутинами.
	portChan := make(chan int, 1)
	errChan := make(chan error, 1)
	rtspUrlChan := make(chan string, 1)
	startedChan := make(chan struct{}) // Уведомление о старте RTSP-сервера.

	// Запускаем RTSP-сервер.
	go func() {
		rtspServer, err := rtspserver.ConfigureRtspServer(portChan, service.Context, service.Logger)
		if err != nil {
			service.Logger.Error("error while setting up RTSP server", "err", err)
			errChan <- err
			close(startedChan) // Уведомляем, что сервер не запустился.
			return
		}

		// Ожидаем завершения запуска RTSP-сервера.
		if err := rtspServer.StartAndWait(); err != nil {
			service.Logger.Error("error while starting RTSP server", "err", err)
			errChan <- err
			return
		}
	}()

	// Обрабатываем результат запуска RTSP-сервера.
	go func() {
		select {
		case port := <-portChan:
			// Сервер успешно запущен. Формируем URL.
			rtspUrl := fmt.Sprintf("%s:%d", service.Envs.RtspStreamUrlPattern, port)
			service.Logger.Debug("RTSP server configured and running", "RTSP_URL", rtspUrl)

			// Публикуем RTSP URL.
			rtspUrlChan <- rtspUrl

			// Уведомляем, что сервер начал запуск.
			close(startedChan)

			// Начинаем трансляцию видео.
			if err := service.VideoService.streamVideoToServer(videoName, rtspUrl); err != nil {
				service.Logger.Error("error while streaming video to server", "err", err)
				service.CtxCancel()
			}
		case err := <-errChan:
			// Обрабатываем ошибку запуска.
			service.Logger.Error("couldn't setup RTSP server => stream wasn't started", "err", err)
		}
	}()

	// Ожидаем уведомления о старте RTSP-сервера.
	<-startedChan
	time.Sleep(time.Second * 1)

	return rtspUrlChan
}
