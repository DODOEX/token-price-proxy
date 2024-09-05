package main

import (
	"time"

	"go.uber.org/fx"

	"github.com/DODOEX/token-price-proxy/internal/application"
	"github.com/DODOEX/token-price-proxy/internal/bootstrap"
	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/module/price"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/scheduler"

	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/DODOEX/token-price-proxy/internal/router"
	fxzerolog "github.com/efectn/fx-zerolog"
	_ "go.uber.org/automaxprocs"
)

// @title                       Go Fiber Starter API Documentation
// @version                     1.0
// @description                 This is a sample API documentation.
// @termsOfService              http://swagger.io/terms/
// @contact.name                Developer
// @contact.email               bangadam.dev@gmail.com
// @license.name                Apache 2.0
// @license.url                 http://www.apache.org/licenses/LICENSE-2.0.html
// @host                        localhost:8080
// @schemes                     http https
// @securityDefinitions.apikey  Bearer
// @in                          header
// @name                        Authorization
// @description                 "Type 'Bearer {TOKEN}' to correctly set the API Key"
// @BasePath                    /
func main() {
	fx.New(
		/* provide patterns */
		// basic
		shared.NewSharedModule,
		scheduler.NewSchedulerModule,
		// application
		fx.Provide(application.NewApplication),
		// database
		fx.Provide(database.NewDatabase),
		// router
		fx.Provide(router.NewRouter),
		//rate limit
		fx.Provide(service.NewRateLimiterService),
		/* provide modules */
		price.NewPriceModule,
		// start aplication
		fx.Invoke(bootstrap.Start),
		// define logger
		fx.WithLogger(fxzerolog.Init()),
		// invoke scheduler tasks
		fx.Invoke(func(s *scheduler.Scheduler) {
			go s.StartCoinsProcessQueue()
			go s.StartCoinHistoricalPriceProcessQueue()
			go s.StartSyncCoins()
			go s.StartSyncCoinsCache()
			go s.StartProcessTopNotifications()
			go s.StartProcessSlackNotifications()
			go s.StartProcessRequestLogs()
			go s.StartProcessDeleteOldData()
		}),
	).Run()

	fx.StartTimeout(10 * time.Minute)
}
