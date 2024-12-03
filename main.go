package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi"

	"rtsp-streamer/configs"
	"rtsp-streamer/external/auth"
	internalHttp "rtsp-streamer/internal/http"
	"rtsp-streamer/internal/service"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	envs := configs.MustConfig()
	minioConfig := configs.MustConfigMinio()
	externalAuthService := configs.MustConfigAuthService()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
	}))

	ctx := context.Background()
	ctxTimeout, cancel := context.WithCancel(ctx)
	defer cancel()

	videoService, err := service.NewVideoService(ctxTimeout, envs, minioConfig, logger)
	if err != nil {
		panic(err)
	}

	err = videoService.CreateBucket(ctxTimeout)
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()

	authRepository := auth.NewAuthRepository(externalAuthService, logger)

	streamerService := service.NewStreamerService(videoService, envs, logger, ctx, cancel)

	webrtcRespository := internalHttp.NewWebrtcRepository(r, streamerService, videoService, authRepository, envs, logger, &ctx)
	if handler, err := webrtcRespository.SetupHandler(r); err == nil {
		logger.Info("server started and running on port :" + envs.ServerPort)
		log.Fatal(http.ListenAndServe(envs.ServerHost+":"+envs.ServerPort, handler))
	}

	log.Fatal(errors.New("failed to start server on port" + envs.ServerPort))
}
