package router

import (
	"github.com/DODOEX/token-price-proxy/internal/module/price"
)

type Router struct {
	PriceRouter *price.PriceRouter
}

func NewRouter(
	priceRouter *price.PriceRouter,

) *Router {
	return &Router{
		PriceRouter: priceRouter,
	}
}

// Register routes
func (r *Router) Register() {
	// Register routes of modules
	r.PriceRouter.RegisterPriceRoutes()
	r.PriceRouter.RegisterCoinsRoutes()
	r.PriceRouter.RegisterAppTokenRoutes()

}
