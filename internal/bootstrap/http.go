package bootstrap

import (
	"context"
	"flag"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"github.com/DODOEX/token-price-proxy/internal/application"
	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/DODOEX/token-price-proxy/internal/router"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

// function to start webserver
func Start(
	lifecycle fx.Lifecycle,
	cfg *koanf.Koanf,
	log zerolog.Logger,
	app *application.Application,
	router *router.Router,
	database *database.Database,
	// amqp *shared.Amqp,
	// etcd *clientv3.Client,
	redis *shared.RedisClient,
) {
	lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				router.Register()

				// ASCII Art
				ascii, err := os.ReadFile("./storage/ascii_art.txt")
				if err != nil {
					log.Debug().Err(err).Msg("An unknown error occurred when to print ASCII art!")
				}

				for _, line := range strings.Split(unsafe.String(unsafe.SliceData(ascii), len(ascii)), "\n") {
					log.Info().Msg(line)
				}

				// Information message
				log.Info().Msg(app.AppName + " is running at the moment!")

				// Debug informations
				if !cfg.Bool("app.production") {
					prefork := "Enabled"
					procs := runtime.GOMAXPROCS(0)
					if !app.Prefork {
						procs = 1
						prefork = "Disabled"
					}

					log.Debug().Msgf("Version: %s", "-")
					log.Debug().Msgf("Hostname: %s", app.Hostname)
					log.Debug().Msgf("Port: %s", app.Port)
					log.Debug().Msgf("Prefork: %s", prefork)
					log.Debug().Msgf("Handlers: %d", app.HandlersCount())
					log.Debug().Msgf("Processes: %d", procs)
					log.Debug().Msgf("PID: %d", os.Getpid())
				}

				// Listen the app (with TLS Support)
				// if cfg.App.TLS.Enable {
				// 	log.Debug().Msg("TLS support was enabled.")

				// 	if err := a.ListenTLS(cfg.App.Port, cfg.App.TLS.CertFile, cfg.App.TLS.KeyFile); err != nil {
				// 		log.Error().Err(err).Msg("An unknown error occurred when to run server!")
				// 	}
				// }

				go func() {
					if err := app.Run(); err != nil {
						log.Error().Err(err).Msg("An unknown error occurred when to run server!")
					}
				}()

				database.ConnectDatabase()

				migrate := flag.Bool("migrate", false, "migrate the database")
				seeder := flag.Bool("seed", false, "seed the database")
				flag.Parse()

				// read flag -migrate to migrate the database
				if *migrate {
					database.MigrateModels()
				}
				// read flag -seed to seed the database
				if *seeder {
					database.SeedModels()
				}

				redis.Connect()
				log.Info().Msgf("2- Connected the Redis succesfully!")

				// amqp.Connect()
				// log.Info().Msgf("3- Connected the Amqp succesfully!")

				return nil
			},
			OnStop: func(ctx context.Context) error {
				log.Info().Msg("Running cleanup tasks...")
				log.Info().Msg("1- Shutdown the Database")
				database.ShutdownDatabase()

				log.Info().Msg("2- Shutdown the Redis")
				if redis != nil {
					redis.Close()
				}

				// log.Info().Msg("3- Shutdown the Amqp")
				// if amqp != nil {
				// 	amqp.Close()
				// }

				// log.Info().Msg("4- Shutdown the ETCD")
				// if etcd != nil {
				// 	etcd.Close()
				// }

				log.Info().Msgf("%s was successful shutdown.", app.AppName)
				log.Info().Msg("\u001b[96msee you again👋\u001b[0m")

				return nil
			},
		},
	)
}
