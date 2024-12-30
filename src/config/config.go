package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	App struct {
		Port int `env:"PORT" envDefault:"3000"`
	}
	DB struct {
		String string `env:"DB_STRING"`
	}
}

func New() (Config, error) {
	cfg := Config{}
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
