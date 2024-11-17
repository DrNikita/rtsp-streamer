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

	videoService, err := internal.NewVideoService(ctxTimeout, envs, minioConfig, logger)
	if err != nil {
		panic(err)
	}

	err = videoService.CreateBucket(ctxTimeout)
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()

	streamerService := internal.NewStreamerService(videoService, envs, logger, ctx, cancel)

	webrtcRespository := internal.NewWebrtcRepository(r, streamerService, videoService, envs, logger, &ctx)
	webrtcRespository.SetupRouter(r)

	logger.Info("server started and running on port :" + envs.ServerPort)
	err = http.ListenAndServe(envs.ServerHost+":"+envs.ServerPort, r)
	if err != nil {
		panic(err)
	}
}
