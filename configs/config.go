package configs

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/kelseyhightower/envconfig"
)

type MinioEnvs struct {
	Endpoint  string `envconfig:"endpoint"`
	Port      string `envconfig:"port"`
	AccessKey string `envconfig:"accesskey"`
	SecretKey string `envconfig:"secretkey"`
	Bucket    string `envconfig:"bucket"`
	SSL       bool   `envconfig:"ssl"`
}

type EnvVariables struct {
	ServerHost                    string `envconfig:"server_host"`
	ServerPort                    string `envconfig:"server_port"`
	VideoSourceDir                string `envconfig:"VIDEO_SOURCE_DIRECTORY"`
	ConvertedVideoContainerPrefix string `envconfig:"VIDEO_CONVERTED_CONTAINER_PREFIX"`
	ConvertedVideoCodecPrefix     string `envconfig:"VIDEO_CONVERTED_CODEC_PREFIX"`
	RtspStreamUrlPattern          string `envconfig:"RTSP_ADDRESS_PATTERN"`
	FfmpegProtocol                string `envconfig:"FFMPEG_PROTOCOL"`
	FfmpegConversionCodec         string `envconfig:"FFMPEG_CONVERSION_CODEC"`
	FfmpegConversionBitrate       string `envconfig:"FFMPEG_CONVERSION_BITRATE"`
	ExternalSetupServerUrl        string `envconfig:"EXTERNAL_SETUP_SERVER_URL"`
	Timeout                       int    `envconfig:"TIMEOUT"`
	WebSocketAddress              string `envconfig:"WEBSOCKET_ADDRESS"`
}

type ExternalAuthService struct {
	VerificationEndpoint string `envconfig:"ENDPOINT_VERIFY_TOKEN"`
	CookieName           string `envconfig:COOKIE_NAME`
}

func MustConfig() *EnvVariables {
	var ev EnvVariables
	err := envconfig.Process("", &ev)
	if err != nil {
		panic(err)
	}
	return &ev
}

func MustConfigMinio() *MinioEnvs {
	var me MinioEnvs
	err := envconfig.Process("minio", &me)
	if err != nil {
		panic(err)
	}
	return &me
}

func MustConfigAuthService() *ExternalAuthService {
	var as ExternalAuthService
	err := envconfig.Process("auth", &as)
	if err != nil {
		panic(err)
	}
	return &as
}
