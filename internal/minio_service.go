package internal

import (
	"context"
	"io"
	"log"
	"log/slog"
	"strings"
	"video-handler/configs"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	RTSP_SERVER_SUPPORTED_CODECS string = "H264,H265,VP8,VP9,MPEG2,MP3,AAC,Opus,PCM,JPEG"
)

type VideoService struct {
	Context     context.Context
	MinioClient *minio.Client
	Envs        *configs.EnvVariables
	MinioEnvs   *configs.MinioEnvs
	Logger      *slog.Logger
}

func NewVideoService(ctx context.Context, envs *configs.EnvVariables, minioEnvs *configs.MinioEnvs, logger *slog.Logger) (*VideoService, error) {
	minioClient, err := GetMinioConnection(minioEnvs.AccessKey, minioEnvs.SecretKey, minioEnvs.Endpoint, minioEnvs.SSL)
	if err != nil {
		return nil, err
	}
	return &VideoService{
		Context:     ctx,
		Envs:        envs,
		MinioEnvs:   minioEnvs,
		Logger:      logger,
		MinioClient: minioClient,
	}, nil
}

func (service *VideoService) streamVideoToServer(sourseVideName, rtspUrl string) error {
	video, err := service.MinioClient.GetObject(service.Context, service.MinioEnvs.Bucket, sourseVideName, minio.GetObjectOptions{})
	if err != nil {
		return err
	}

	_, err = service.StreamVideoAsRTSP(video, service.Envs.FfmpegProtocol, rtspUrl)
	if err != nil {
		return err
	}

	service.Logger.Info("video successfully downloded from minio")
	return nil
}

func extractFileNameComponents(fileName string) (string, string) {
	fileComponents := strings.Split(fileName, ".")
	if len(fileComponents) > 1 {
		return strings.Join(fileComponents[:len(fileComponents)-1], ""), fileComponents[len(fileComponents)-1]
	}
	return "", ""
}

func GetMinioConnection(accessKey, secretKey, endpoint string, ssl bool) (*minio.Client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: ssl,
	})
	if err != nil {
		log.Fatalln(err)
	}

	return minioClient, nil
}

func (service *VideoService) CreateBucket(ctx context.Context) error {
	exists, err := service.MinioClient.BucketExists(context.Background(), service.MinioEnvs.Bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	err = service.MinioClient.MakeBucket(context.Background(), service.MinioEnvs.Bucket, minio.MakeBucketOptions{
		ObjectLocking: true,
	})
	return err
}

func (service *VideoService) UploadVideo(video io.Reader, videoName string) (minio.UploadInfo, error) {
	return service.MinioClient.PutObject(service.Context, service.MinioEnvs.Bucket, videoName, video, -1, minio.PutObjectOptions{
		ContentType: "video/mp4",
	})
}

func (service *VideoService) DeleteVideo(videoName string) error {
	return service.MinioClient.RemoveObject(context.Background(), service.MinioEnvs.Bucket, videoName, minio.RemoveObjectOptions{})
}

func (service *VideoService) GetVideoList() ([]string, error) {
	service.Logger.Info("Getting video list from Minio bucket", "bucket", service.MinioEnvs.Bucket)
	objects := service.MinioClient.ListObjects(context.Background(), service.MinioEnvs.Bucket, minio.ListObjectsOptions{
		WithMetadata: true,
	})

	var videos []string
	for obj := range objects {
		videos = append(videos, obj.Key)
	}

	service.Logger.Info("Video list obtained from Minio bucket", "bucket", service.MinioEnvs.Bucket, "videos", videos)
	return videos, nil
}

func (service *VideoService) GetVideo(videoName string) (*minio.Object, error) {
	return service.MinioClient.GetObject(service.Context, service.MinioEnvs.Bucket, videoName, minio.GetObjectOptions{})
}
