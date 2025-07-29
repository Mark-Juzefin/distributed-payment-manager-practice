package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	App struct {
		Port int `env:"PORT" envDefault:"3000"`
	}
	PG struct {
		String  string `env:"PG_URL"`
		PoolMax string `env:"PG_POOL_MAX" envDefault:"10"`
	}
	Log struct {
		Level string `env:"LOG_LEVEL" envDefault:"info"`
	}
}

func New() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
