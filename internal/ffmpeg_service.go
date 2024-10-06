package internal

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"strings"

	cmdCommand "video-handler/pkg"

	"github.com/minio/minio-go/v7"
)

const (
	supportedCodecs string = "H265,H264,VP9,VP8"
)

func (service *VideoService) processVideoContainer(video multipart.File, videoInfo *multipart.FileHeader) (bool, error) {
	var conversionNeed bool

	videoCodec, err := service.getVideoCodec(video)
	if err != nil {
		service.Logger.Error("error getting video codec", "err", err.Error())
		return conversionNeed, err
	}

	if videoCodec == "" || !strings.Contains(strings.ToLower(supportedCodecs), strings.ToLower(videoCodec)) {
		errChan := make(chan error)
		go func() {
			defer close(errChan)
			video.Seek(0, 0)
			outputVideo, err := service.ConvertVideoCodec(video, service.Envs.FfmpegConversionCodec, service.Envs.FfmpegConversionBitrate)
			if err != nil {
				service.Logger.Error(ErrorExecutingFfmpegCommand, "err", err.Error())
			}

			uploadInfo, err := service.UploadVideo(outputVideo, videoInfo.Filename)
			if err != nil {
				errChan <- err
			}

			service.Logger.Info("video uploaded successfully", "video_name", uploadInfo.Key, "video_size", uploadInfo.Size)

			errChan <- nil
		}()

		go func() {
			err := <-errChan
			if err != nil {
				service.Logger.Error("error converting video", "err", err.Error())
			}
		}()

		conversionNeed = true
	}

	return conversionNeed, nil
}

func (service *VideoService) StreamVideoAsRTSP(video *minio.Object, protocol, streamAddress string) ([]byte, error) {
	service.Logger.Debug("", video)
	rtspVidoStreamCommand := cmdCommand.CmdCommand{
		App:    "ffmpeg",
		Args:   []string{"-re", "-stream_loop", "-1", "-i", "pipe:0", "-c", "copy", "-bsf:v", "h264_mp4toannexb", "-f", protocol, streamAddress},
		Pipe:   video,
		Logger: *service.Logger,
	}

	stdout, err := rtspVidoStreamCommand.ExecuteCommand()
	if err != nil {
		rtspVidoStreamCommand.Logger.Error("error starting video as rtsp stream", "error msg", err.Error())
		return nil, err
	}

	service.Logger.Info("video steram started", "stdout", string(stdout))
	return stdout, nil
}

func (service *VideoService) ConvertVideoCodec(video io.ReadSeekCloser, outputVideoCodec, bitrate string) (io.Reader, error) {
	videoCodecConvertingCommand := cmdCommand.CmdCommand{
		App:    "ffmpeg",
		Args:   []string{"-i", "pipe:0", "-c:v", outputVideoCodec, "-crf", bitrate, "-f", "mpegts", "pipe:1"},
		Pipe:   video,
		Logger: *service.Logger,
	}

	ffmpegStdout, err := videoCodecConvertingCommand.ExecuteWithPipeCreation()
	if err != nil {
		videoCodecConvertingCommand.Logger.Error("error converting videocodec", "msg", err.Error())
		return nil, err
	}

	return ffmpegStdout, nil
}

func (service *VideoService) getVideoCodec(video io.ReadSeekCloser) (string, error) {
	videoCodecDefinictionCommand := cmdCommand.CmdCommand{
		App:    "ffprobe",
		Args:   []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", "pipe:0"},
		Pipe:   video,
		Logger: *service.Logger,
	}

	stdout, err := videoCodecDefinictionCommand.ExecuteCommand()
	if err != nil {
		videoCodecDefinictionCommand.Logger.Error("error getting video codec", "msg", err.Error())
		return "", err
	}
	if stdout == nil {
		service.Logger.Error("couldn't get video codec", "codec_value", string(stdout))
		return "", fmt.Errorf("cudn't get video codec")
	}

	videoCodec := string(stdout)

	videoCodecDefinictionCommand.Logger.Info("video codec received", "value", videoCodec)

	return videoCodec, nil
}

func (service *VideoService) getVideoContainers(video io.ReadSeekCloser) (string, error) {
	videoContainerDefenitionCommand := cmdCommand.CmdCommand{
		App:    "ffprobe",
		Args:   []string{"-v", "quiet", "-show_entries", "format=format_name", "-of", "default=noprint_wrappers=1:nokey=1", "pipe:0"},
		Pipe:   video,
		Logger: *service.Logger,
	}

	ffmpegStdout, err := videoContainerDefenitionCommand.ExecuteCommand()
	if err != nil {
		return "", err
	}

	videoContainers := string(ffmpegStdout)

	service.Logger.Info("video containers was got", "containers", videoContainers)

	return videoContainers, nil
}

func (service *VideoService) ConvertVideoExtension(inputVideo io.ReadSeekCloser) (io.ReadCloser, error) {
	convertVideoExtentionCommand := cmdCommand.CmdCommand{
		App:    "ffmpeg",
		Args:   []string{"-i", "pipe:0", "-f", "mpegts", "pipe:1"},
		Pipe:   inputVideo,
		Logger: *service.Logger,
	}

	ffmpegStdout, err := convertVideoExtentionCommand.ExecuteWithPipeCreation()
	if err != nil {
		convertVideoExtentionCommand.Logger.Error("error converting video extension", "msg", err.Error())
		return nil, err
	}

	return ffmpegStdout, nil
}

func RTSPtoHLSconverter(rtspUrl string, logger *slog.Logger) ([]byte, error) {
	convertVideoExtentionCommand := cmdCommand.CmdCommand{
		App:    "ffmpeg",
		Args:   []string{"-i", rtspUrl, "-c:v", "copy", "-c:a", "copy", "-hls_time", "2", "-hls_list_size", "10", "-hls_flags", "delete_segments", "-start_number", "1", "output.m3u8"},
		Logger: *logger,
	}

	stdout, err := convertVideoExtentionCommand.ExecuteCommand()
	if err != nil {
		convertVideoExtentionCommand.Logger.Error("error converting video extention", "msg", err.Error())
		return nil, err
	}

	return stdout, err
}
