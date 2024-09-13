package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"video-handler/configs"
	"video-handler/internal/rtspserver"

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
	r.Post("/stream", respository.stream)
	r.Post("/upload", respository.upload)
	r.Get("/video-list", respository.videoList)
	// r.Get("/video", respository.video)
}

func (repository *HttpRepository) stream(w http.ResponseWriter, r *http.Request) {
	freePort, err := findFreePort()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		wg.Done()
		rtspServer := rtspserver.ConfigureRtspServer(":"+strconv.Itoa(freePort), repository.Context)
		err := rtspServer.StartAndWait()
		if err != nil {
			repository.CtxCancel()
		}
	}()

	wg.Wait()

	rtspUrl := fmt.Sprintf("%s:%d", repository.Envs.RtspStreamUrlPattern, freePort)

	var responseBody struct {
		VideoName string `json:"source_video"`
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	err = json.Unmarshal(bodyBytes, &responseBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	wg.Add(1)
	go func() {
		wg.Done()
		err = repository.Service.stream(responseBody.VideoName, rtspUrl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}()

	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		RtspUrl string `json:"rtsp_url"`
	}{
		RtspUrl: rtspUrl,
	})
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
