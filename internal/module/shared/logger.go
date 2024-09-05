package shared

import (
	"os"

	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp/prefork"
)

// initialize logger
func NewLogger(cfg *koanf.Koanf) zerolog.Logger {
	zerolog.TimeFieldFormat = cfg.String("logger.time-format")

	if cfg.Get("logger.prettier") != nil {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	zerolog.SetGlobalLevel(zerolog.Level(int8(cfg.Int("logger.level"))))

	return log.Hook(PreforkHook{})
}

// prefer hook for zerologger
type PreforkHook struct{}

func (h PreforkHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	if prefork.IsChild() {
		e.Discard()
	}
}
