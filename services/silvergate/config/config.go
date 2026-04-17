package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Port     int    `env:"PORT" envDefault:"3002"`
	PgURL    string `env:"SILVERGATE_PG_URL" required:"true"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Merchant webhook callback URL
	WebhookCallbackURL string `env:"WEBHOOK_CALLBACK_URL" required:"true"`

	// Mock acquirer settings
	AcquirerAuthApproveRate   float64       `env:"ACQUIRER_AUTH_APPROVE_RATE" envDefault:"0.9"`
	AcquirerSettleSuccessRate float64       `env:"ACQUIRER_SETTLE_SUCCESS_RATE" envDefault:"0.95"`
	AcquirerSettleDelay       time.Duration `env:"ACQUIRER_SETTLE_DELAY" envDefault:"500ms"`
}

func New() (Config, error) {
	return env.ParseAs[Config]()
}
