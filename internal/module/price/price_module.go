package price

import (
	"github.com/DODOEX/token-price-proxy/internal/application"
	"github.com/DODOEX/token-price-proxy/internal/module/price/controller"
	"github.com/DODOEX/token-price-proxy/internal/module/price/middleware"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

// struct of AgentRouter
type PriceRouter struct {
	App                *application.Application
	Controller         *controller.Controller
	RateLimiterService *service.RateLimiterService
	Logger             zerolog.Logger
}

// register bulky of agent module
var NewPriceModule = fx.Options(
	// register repository of agent module
	fx.Provide(repository.NewCoinRepository),
	fx.Provide(repository.NewCoinHistoricalPriceRepository),
	fx.Provide(repository.NewAppTokenRepository),
	fx.Provide(repository.NewRequestLogRepository),
	fx.Provide(repository.NewSlackNotificationRepository),

	fx.Provide(service.NewCoinGeckoService),
	fx.Provide(service.NewGeckoTerminalService),
	fx.Provide(service.NewDefiLlamaService),
	fx.Provide(service.NewDodoexRouteService),
	fx.Provide(service.NewPriceService),
	fx.Provide(service.NewCoinsService),
	fx.Provide(service.NewAppTokenService),
	fx.Provide(service.NewCoinGeckoOnChainService),
	fx.Provide(service.NewSlackNotificationService),

	// register controller of agent module
	fx.Provide(controller.NewController),

	fx.Provide(NewPriceRouter),

	// 这里添加 CoinChecker 的实现
	fx.Provide(func(repo repository.CoinRepository) shared.CoinChecker {
		return repo
	}),
)

// init AgentRouter
func NewPriceRouter(app *application.Application, controller *controller.Controller, rateLimiterService *service.RateLimiterService, logger zerolog.Logger) *PriceRouter {
	return &PriceRouter{
		App:                app,
		Controller:         controller,
		RateLimiterService: rateLimiterService,
		Logger:             logger,
	}
}

// register routes of agent module
func (_i *PriceRouter) RegisterPriceRoutes() {
	// define controllers
	priceController := _i.Controller.Price

	rateLimitMiddleware := middleware.RateLimitMiddleware(_i.RateLimiterService, _i.Logger)

	// define routes
	_i.App.Router.GET("/price", rateLimitMiddleware(priceController.GetPrice))
	_i.App.Router.GET("/price/coins", rateLimitMiddleware(priceController.GetCoinList))
	_i.App.Router.GET("/price/sync", rateLimitMiddleware(priceController.SyncCoins))
	_i.App.Router.ANY("/api/v1/price/current/batch", rateLimitMiddleware(priceController.GetBatchPrice))
	_i.App.Router.ANY("/api/v1/price/historical/batch", rateLimitMiddleware(priceController.GetBatchHistoricalPrice))
	_i.App.Router.ANY("/api/v1/price/current", rateLimitMiddleware(priceController.GetPrice))
	_i.App.Router.ANY("/api/v1/price/historical", rateLimitMiddleware(priceController.GetHistoricalPrice))
}

func (_i *PriceRouter) RegisterCoinsRoutes() {
	coinsController := _i.Controller.Coins

	_i.App.Router.POST("/coins/add", coinsController.AddCoin)
	_i.App.Router.POST("/coins/update/{id}", coinsController.UpdateCoin)
	_i.App.Router.POST("/coins/delete/{id}", coinsController.DeleteCoin)
	_i.App.Router.POST("/redis/delete/{key}", coinsController.DeleteRedisKey)
	_i.App.Router.GET("/coins/{id}", coinsController.GetCoinByID)
	_i.App.Router.GET("/coins/refresh", coinsController.RefreshAllCoinsCache)
	_i.App.Router.POST("/coins/refreshList", coinsController.RefreshCoinListCache)
}

func (_i *PriceRouter) RegisterAppTokenRoutes() {
	tokenController := _i.Controller.Token

	_i.App.Router.POST("/appToken/add", tokenController.AddAppToken)
	_i.App.Router.POST("/appToken/update/{token}", tokenController.UpdateAppToken)
	_i.App.Router.POST("/appToken/delete/{token}", tokenController.DeleteAppToken)
	_i.App.Router.GET("/appToken/{token}", tokenController.GetAppToken)
	_i.App.Router.GET("/appToken", tokenController.GetAllAppTokens)

	_i.App.Router.GET("/k8s/healthz", tokenController.CheckhHealthz)

}
