package service

import (
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
)

type AppTokenService interface {
	GetAppTokenByToken(token string) (*schema.AppToken, error)
	GetAllAppTokens() ([]schema.AppToken, error)
	AddAppToken(appToken *schema.AppToken) error
	UpdateAppToken(appToken *schema.AppToken) error
	DeleteAppToken(token string) error
}

type appTokenService struct {
	appTokenRepo repository.AppTokenRepository
}

func NewAppTokenService(appTokenRepo repository.AppTokenRepository) AppTokenService {
	return &appTokenService{
		appTokenRepo: appTokenRepo,
	}
}

func (s *appTokenService) GetAppTokenByToken(token string) (*schema.AppToken, error) {
	return s.appTokenRepo.GetAppTokenByToken(token)
}

func (s *appTokenService) GetAllAppTokens() ([]schema.AppToken, error) {
	return s.appTokenRepo.GetAllAppTokens()
}

func (s *appTokenService) AddAppToken(appToken *schema.AppToken) error {
	return s.appTokenRepo.AddAppToken(appToken)
}

func (s *appTokenService) UpdateAppToken(appToken *schema.AppToken) error {
	return s.appTokenRepo.UpdateAppToken(appToken)
}

func (s *appTokenService) DeleteAppToken(token string) error {
	return s.appTokenRepo.DeleteAppToken(token)
}
