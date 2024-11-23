package auth

import (
	"log/slog"
	"video-handler/configs"
)

type Authentificatior interface {
	testfunc() error
}

type AuthRepository struct {
	authConfig *configs.ExternalAuthService
	logger     *slog.Logger
}

func NewAuthRepository(authConfig *configs.ExternalAuthService, logger *slog.Logger) *AuthRepository {
	return &AuthRepository{
		authConfig: authConfig,
		logger:     logger,
	}
}

func (ar *AuthRepository) testfunc() error {
	return nil
}
