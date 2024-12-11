package auth

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
	"video-handler/configs"
)

type Authentificatior interface {
	VerifyCredentials(next http.Handler) http.Handler
}

type AuthRepository struct {
	configs *configs.ExternalAuthService
	logger  *slog.Logger
}

func NewAuthRepository(authConfig *configs.ExternalAuthService, logger *slog.Logger) *AuthRepository {
	return &AuthRepository{
		configs: authConfig,
		logger:  logger,
	}
}

func (mr *AuthRepository) VerifyCredentials(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jsonBody := []byte(`{"client_message": "hello, server!"}`)
		bodyReader := bytes.NewReader(jsonBody)
		verifyRequest, err := http.NewRequest(http.MethodPost, mr.configs.VerificationEndpoint, bodyReader)
		if err != nil {
			mr.logger.Error("Bad!!!", "err", err)
		}

		if token, err := r.Cookie(mr.configs.VerificationEndpoint); err != nil {
			verifyRequest.AddCookie(token)
		}

		verifyRequest.Header.Set("Content-Type", "application/json")

		client := http.Client{
			Timeout: 30 * time.Second,
		}

		verifyResponse, err := client.Do(verifyRequest)
		if err != nil {
			fmt.Printf("client: error making http request: %s\n", err)
			os.Exit(1)
		}

		if verifyResponse.Status != string(http.StatusOK) {
		}

		next.ServeHTTP(w, r)
	})
}
