package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
		verifyRequest, err := http.NewRequest(http.MethodPost, mr.configs.VerificationEndpoint, http.NoBody)
		if err != nil {
			mr.logger.Error("Bad!!!", "err", err)
			return
		}

		http.Post("", "", nil)

		accessToken, err := r.Cookie(mr.configs.AccessTokenCookieName)
		if err != nil {
			mr.logger.Error("failed to get cookie access token", "err", err)
		}

		refreshToken, err := r.Cookie(mr.configs.RefreshTokenCookieName)
		if err != nil {
			mr.logger.Error("failed to get cookie refresh token", "err", err)
		}

		if accessToken != nil {
			verifyRequest.AddCookie(accessToken)
		}

		if refreshToken != nil {
			verifyRequest.AddCookie(refreshToken)
		}

		verifyRequest.Header.Set("Content-Type", "application/json")

		client := http.Client{
			Timeout: 30 * time.Second,
		}

		verifyResponse, err := client.Do(verifyRequest)
		if err != nil {
			fmt.Printf("client: error making http request: %s\n", err)
		}

		if !strings.Contains(verifyResponse.Status, "200") &&
			!strings.Contains(verifyResponse.Status, "201") {
			http.Redirect(w, r, mr.configs.LoginPageURL, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
