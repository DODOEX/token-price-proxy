package service

import (
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
)

type CoinsService interface {
	AddCoin(coin schema.Coins) error
	UpdateCoin(id string, updatedCoin schema.Coins) error
	DeleteCoin(id string) error
	GetCoinByID(id string) (*schema.Coins, error)
	RefreshAllCoinsCache() error
	RefreshCoinListCache(ids []string) error
}

type coinsService struct {
	coinsRepo repository.CoinRepository
}

func NewCoinsService(coinsRepo repository.CoinRepository) CoinsService {
	return &coinsService{
		coinsRepo: coinsRepo,
	}
}

func (s *coinsService) AddCoin(coin schema.Coins) error {
	return s.coinsRepo.UpsertCoins([]schema.Coins{coin})
}

func (s *coinsService) UpdateCoin(id string, updatedCoin schema.Coins) error {
	updatedCoin.ID = id
	return s.coinsRepo.UpsertCoins([]schema.Coins{updatedCoin})
}

func (s *coinsService) DeleteCoin(id string) error {
	return s.coinsRepo.DeleteCoinByID(id)
}
func (s *coinsService) RefreshAllCoinsCache() error {
	return s.coinsRepo.RefreshAllCoinsCache()
}

func (s *coinsService) RefreshCoinListCache(ids []string) error {
	return s.coinsRepo.RefreshCoinListCache(ids)
}

func (s *coinsService) GetCoinByID(id string) (*schema.Coins, error) {
	return s.coinsRepo.GetCoinsByOneID(id)
}
