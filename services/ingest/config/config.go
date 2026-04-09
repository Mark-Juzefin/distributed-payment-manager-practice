package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds Ingest service configuration.
type Config struct {
	Port     int    `env:"PORT" envDefault:"3001"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Webhook processing mode: "kafka" (async via Kafka) or "http" (sync via HTTP to API)
	WebhookMode string `env:"WEBHOOK_MODE" envDefault:"kafka"`

	// Kafka configuration (required for kafka mode)
	KafkaBrokers       []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaOrdersTopic   string   `env:"KAFKA_ORDERS_TOPIC" envDefault:"webhooks.orders"`
	KafkaDisputesTopic string   `env:"KAFKA_DISPUTES_TOPIC" envDefault:"webhooks.disputes"`

	// Inbox mode configuration (required for inbox mode)
	PgURL     string `env:"INGEST_PG_URL"`
	PgPoolMax int    `env:"INGEST_PG_POOL_MAX" envDefault:"5"`

	// HTTP mode configuration (required for http and inbox modes)
	APIBaseURL        string        `env:"API_BASE_URL" envDefault:"http://localhost:3000"`
	APITimeout        time.Duration `env:"API_TIMEOUT" envDefault:"10s"`
	APIRetryAttempts  int           `env:"API_RETRY_ATTEMPTS" envDefault:"3"`
	APIRetryBaseDelay time.Duration `env:"API_RETRY_BASE_DELAY" envDefault:"100ms"`
	APIRetryMaxDelay  time.Duration `env:"API_RETRY_MAX_DELAY" envDefault:"5s"`

	// Inbox worker configuration (required for inbox mode)
	InboxPollInterval time.Duration `env:"INBOX_POLL_INTERVAL" envDefault:"100ms"`
	InboxBatchSize    int           `env:"INBOX_BATCH_SIZE" envDefault:"10"`
	InboxMaxRetries   int           `env:"INBOX_MAX_RETRIES" envDefault:"5"`
}

// New parses environment variables for the Ingest service.
func New() (Config, error) {
	return env.ParseAs[Config]()
}
