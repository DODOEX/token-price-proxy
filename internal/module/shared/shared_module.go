package shared

import (
	"go.uber.org/fx"
)

var NewSharedModule = fx.Options(
	// fx.Provide(NewEtcdClient),
	fx.Provide(NewKoanfInstance),
	fx.Provide(NewLogger),
	fx.Provide(NewRedisClient),
	fx.Invoke(LoadEnv),
	fx.Provide(NewCoinsThrottler),
	// fx.Provide(NewRabbitMQ),
)
