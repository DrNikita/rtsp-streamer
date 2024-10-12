package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi"

	"video-handler/configs"
	"video-handler/internal"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	envs := configs.MustConfig()
	minioConfig := configs.MustConfigMinio()

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
	}))

	ctx := context.Background()
	ctxTimeout, cancel := context.WithCancel(ctx)
	defer cancel()

	service, err := internal.NewVideoService(ctxTimeout, envs, minioConfig, logger)
	if err != nil {
		panic(err)
	}

	err = service.CreateBucket(ctxTimeout)
	if err != nil {
		panic(err)
	}

	streamerService := internal.NewStreamerService(service, envs, logger, ctx, cancel)
	webrtcRespository := internal.NewWebrtcRepository(streamerService, logger)

	httpRepository := &internal.HttpRepository{
		VideoService:     service,
		WebrtcRepository: webrtcRespository,
		Config:           envs,
		Logger:           logger,
		Context:          ctxTimeout,
		CtxCancel:        cancel,
	}

	r := chi.NewRouter()

	httpRepository.RegisterRoutes(r)

	logger.Info("server started and running on port :" + envs.ServerPort)
	err = http.ListenAndServe(envs.ServerHost+":"+envs.ServerPort, r)
	if err != nil {
		panic(err)
	}
}
