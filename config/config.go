package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port      int    `env:"PORT" envDefault:"3000"`
	PgURL     string `env:"PG_URL" required:"true"`
	PgPoolMax int    `env:"PG_POOL_MAX" envDefault:"10"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
}

func New() (Config, error) {
	c, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, err
	}

	// Debug output
	fmt.Printf("DEBUG: PG_POOL_MAX = %d\n", c.PgPoolMax)
	fmt.Printf("DEBUG: PG_URL = %s\n", c.PgURL)
	return c, nil
}
