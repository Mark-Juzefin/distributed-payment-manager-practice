package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port      int    `env:"PORT" envDefault:"3000"`
	PgURL     string `env:"PG_URL" required:"true"`
	PgPoolMax int    `env:"PG_POOL_MAX" envDefault:"10"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`

	SilvergateBaseURL                 string        `env:"SILVERGATE_BASE_URL" required:"true"`
	SilvergateSubmitRepresentmentPath string        `env:"SILVERGATE_SUBMIT_REPRESENTMENT_PATH" required:"true"`
	HTTPSilvergateClientTimeout       time.Duration `env:"HTTP_SILVERGATE_CLIENT_TIMEOUT" envDefault:"20s"`
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
