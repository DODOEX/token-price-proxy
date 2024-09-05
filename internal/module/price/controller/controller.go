package controller

import (
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
)

type Controller struct {
	Price PriceController
	Coins CoinsController
	Token AppTokenController
}

func NewController(
	priceService service.PriceService,
	coingeckoService service.CoinGeckoService,
	coinsService service.CoinsService,
	appTokenService service.AppTokenService,
	requestLogRepo repository.RequestLogRepository,
	redisClient *shared.RedisClient,
	logger zerolog.Logger) *Controller {
	return &Controller{
		Price: NewPriceController(priceService, coingeckoService, requestLogRepo, logger),
		Coins: NewCoinsController(coinsService, redisClient),
		Token: NewAppTokenController(appTokenService),
	}
}
