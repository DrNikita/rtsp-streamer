package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
	"video-handler/configs"

	"github.com/go-chi/chi"
)

type Response struct {
	Status       int
	IsConverting bool
	Result       any
	Error        string
}

type HttpRepository struct {
	VideoService     *VideoService
	WebrtcRepository *WebrtcRepository
	Config           *configs.EnvVariables
	Logger           *slog.Logger
	Context          context.Context
	CtxCancel        context.CancelFunc
}

func NewHttpRepository() {
}

func (repository *HttpRepository) RegisterRoutes(r chi.Router) {
	r.Post("/upload", repository.upload)
	r.Get("/video-list", repository.videoList)
	r.HandleFunc("/websocket", repository.WebrtcRepository.websocketHandler)

	/*
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if err := repository.WebrtcRepository.indexTemplate.Execute(w, repository.Config.WebSocketAddress); err != nil {
				log.Fatal(err)
			}
		})
	*/

	go func() {
		for range time.NewTicker(time.Second * 1).C {
			repository.WebrtcRepository.dispatchKeyFrame()
		}
	}()
}

func (repository *HttpRepository) upload(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(1000000)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	buffer, handler, err := r.FormFile("video")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer buffer.Close()

	conversionNeed, err := repository.VideoService.processVideoContainer(buffer, handler)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Status:       http.StatusBadRequest,
			IsConverting: false,
			Error:        err.Error(),
		})
		return
	}

	buffer.Seek(0, 0)

	if !conversionNeed {
		uploadInfo, err := repository.VideoService.UploadVideo(buffer, handler.Filename)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		repository.Logger.Info("video doesn't need conversion and was updloaded successfully", "video_name", uploadInfo.Key, "video_size", uploadInfo.Size)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Status:       http.StatusOK,
			IsConverting: true,
			Result:       uploadInfo,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Status:       http.StatusOK,
		IsConverting: false,
		Result:       "vide uploaded successfully",
	})
}

func (repository *HttpRepository) videoList(w http.ResponseWriter, r *http.Request) {
	videos, err := repository.VideoService.GetVideoList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(videos)
}

