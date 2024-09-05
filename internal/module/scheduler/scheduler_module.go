package scheduler

import (
	"go.uber.org/fx"
)

var NewSchedulerModule = fx.Options(
	fx.Provide(NewScheduler),
)
