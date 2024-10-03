package internal

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"video-handler/configs"

	"github.com/go-chi/chi"
)

type HttpRepository struct {
	Service   *VideoService
	Envs      *configs.EnvVariables
	Logger    *slog.Logger
	Context   context.Context
	CtxCancel context.CancelFunc
}

func (respository *HttpRepository) RegisterRoutes(r chi.Router) {
	r.Post("/upload", respository.upload)
	r.Get("/video-list", respository.videoList)
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

	conversionNeed, err := repository.Service.processVideoContainer(buffer, handler)
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
		uploadInfo, err := repository.Service.UploadVideo(buffer, handler.Filename)
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
	videos, err := repository.Service.GetVideoList()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	json.NewEncoder(w).Encode(videos)
}

/*
func (repository *HttpRepository) video(w http.ResponseWriter, r *http.Request) {
	videoName := r.URL.Query().Get("videoName")
	video, err := repository.Service.GetVideo(videoName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer video.Close()

	cmd := exec.Command("ffmpeg", "-i", "pipe:0", "-c:v", "libx264", "SUCCESS111111.mp4")
	cmd.Stdin = video

	err = cmd.Run()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.ServeFile(w, r, "SUCCESS111111.mp4")
}
*/
