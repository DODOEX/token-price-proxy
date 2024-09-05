package application

import (
	"time"

	"github.com/DODOEX/token-price-proxy/utils/config"
	"github.com/fasthttp/router"
	"github.com/knadh/koanf/v2"
	"github.com/valyala/fasthttp"
)

type Application struct {
	AppName               string
	Network               string
	Hostname              string
	Port                  string
	Prefork               bool
	IdleTimeout           time.Duration
	EnablePrintRoutes     bool
	DisableStartupMessage bool
	Router                *router.Router
	s                     *fasthttp.Server
}

func NewApplication(cfg *koanf.Koanf) *Application {
	var network string
	if cfg.Get("app.network") != nil {
		network = cfg.String("app.network")
	}
	hostname, port := config.ParseAddress(cfg.String("app.host"))
	if hostname == "" {
		if network == "tcp6" {
			hostname = "[::1]"
		} else {
			hostname = "0.0.0.0"
		}
	}
	application := &Application{
		Network:               network,
		Hostname:              hostname,
		Port:                  port,
		AppName:               cfg.String("app.name"),
		Prefork:               cfg.Bool("app.prefork"),
		IdleTimeout:           cfg.Duration("app.idle-timeout") * time.Second,
		EnablePrintRoutes:     cfg.Bool("app.print-routes"),
		DisableStartupMessage: true,
		Router:                router.New(),
	}

	return application
}

func (a *Application) HandlersCount() int {
	// return int(a.s.GetOpenConnectionsCount())
	return 0
}

func (a *Application) Run() error {
	a.s = &fasthttp.Server{
		Handler:         a.Router.Handler,
		ReadBufferSize:  4096 * 20,
		WriteBufferSize: 4096 * 20,
	}
	return a.s.ListenAndServe(a.Hostname + ":" + a.Port)
}
