package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp/prefork"
	"gopkg.in/yaml.v3"
)

// app struct config
type app = struct {
	Name        string        `yaml:"name"`
	Port        string        `yaml:"port"`
	PrintRoutes bool          `yaml:"print-routes"`
	Prefork     bool          `yaml:"prefork"`
	Production  bool          `yaml:"production"`
	IdleTimeout time.Duration `yaml:"idle-timeout"`
	TLS         struct {
		Enable   bool   `yaml:"enable"`
		CertFile string `yaml:"cert-file"`
		KeyFile  string `yaml:"key-file"`
	}
}

// db struct config
type db = struct {
	Gorm struct {
		DisableForeignKeyConstraintWhenMigrating bool `yaml:"disable-foreign-key-constraint-when-migrating"`
	}
	Postgres struct {
		DSN string `yaml:"dsn"`
	}
}

// log struct config
type logger = struct {
	TimeFormat string        `yaml:"time-format"`
	Level      zerolog.Level `yaml:"level"`
	Prettier   bool          `yaml:"prettier"`
}

// middleware
type middleware = struct {
	Compress struct {
		Enable bool
		Level  int
	}

	Recover struct {
		Enable bool
	}

	Monitor struct {
		Enable bool
		Path   string
	}

	Pprof struct {
		Enable bool
	}

	Limiter struct {
		Enable     bool
		Max        int
		Expiration time.Duration `yaml:"expiration_seconds"`
	}

	FileSystem struct {
		Enable bool
		Browse bool
		MaxAge int `yaml:"max_age"`
		Index  string
		Root   string
	}

	Jwt struct {
		Secret     string        `yaml:"secret"`
		Expiration time.Duration `yaml:"expiration_seconds"`
	}
}

type EndpointInfo struct {
	Url        string             `yaml:"url" koanf:"url"`
	Headers    *map[string]string `yaml:"headers" koanf:"headers"`
	TslVersion *string            `yaml:"tsl-version" koanf:"tsl-version"`
	Weight     *int               `yaml:"weight" koanf:"weight"`
}

type ChainGroup = struct {
	Algorithm string          `yaml:"algorithm,omitempty" koanf:"algorithm,omitempty"`
	Endpoints []*EndpointInfo `yaml:"endpoints,omitempty" koanf:"endpoints,omitempty"`
}

type ChainServices = struct {
	Activenode ChainGroup `yaml:"activenode" koanf:"activenode"`
	Fullnode   ChainGroup `yaml:"fullnode" koanf:"fullnode"`
}

type Chain = struct {
	ChainID   int    `yaml:"id" koanf:"id"`
	ChainCode string `yaml:"code" koanf:"code"`
	// ChainGroup
	Algorithm string          `yaml:"algorithm,omitempty" koanf:"algorithm,omitempty"`
	Endpoints []*EndpointInfo `yaml:"endpoints,omitempty" koanf:"endpoints,omitempty"`

	Services *ChainServices `yaml:"services,omitempty" koanf:"services,omitempty"`
}

// type Chain = ChainServices | ChainGroup;

type Config struct {
	App        app
	DB         db
	Logger     logger
	Middleware middleware
	Chains     []map[string]any `yaml:"loadbalances" koanf:"loadbalances"`
	APIKey     apiKey           `yaml:"apiKey"`
}

// func to parse config
func ParseConfig(file []byte) (*Config, error) {
	var (
		contents *Config
		err      error
	)
	err = yaml.Unmarshal(file, &contents)

	return contents, err
}

func ReadAndParseConfig(filename string, debug ...bool) (*Config, error) {
	var (
		file []byte
		err  error
	)

	if len(debug) > 0 {
		file, err = os.ReadFile(filename)
	} else {
		_, b, _, _ := runtime.Caller(0)
		// get base path
		path := filepath.Dir(filepath.Dir(filepath.Dir(b)))
		file, err = os.ReadFile(filepath.Join(path, "./config/", filename))
	}

	if err != nil {
		return &Config{}, err
	}

	return ParseConfig(file)
}

// initialize config
func NewConfig() *Config {
	var filename string = "default.yaml"
	config, err := ReadAndParseConfig(filename)
	if err != nil && !prefork.IsChild() {
		// panic if config is not found
		log.Panic().Err(err).Msg("'" + filename + "' not found")
	}

	return config
}

// func to parse address
func ParseAddress(raw string) (hostname, port string) {
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		return raw[:i], raw[i+1:]
	}

	return raw, ""
}

type apiKey struct {
    CoinGecko string `yaml:"coingecko"`
}