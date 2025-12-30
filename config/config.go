package config

import (
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
	SilvergateCapturePath             string        `env:"SILVERGATE_CAPTURE_PATH" required:"true"`
	HTTPSilvergateClientTimeout       time.Duration `env:"HTTP_SILVERGATE_CLIENT_TIMEOUT" envDefault:"20s"`

	OpensearchUrls []string `env:"OPENSEARCH_URLS" required:"true"`

	OpensearchIndexDisputes string `env:"OPENSEARCH_INDEX_DISPUTES" required:"true"`
	OpensearchIndexOrders   string `env:"OPENSEARCH_INDEX_ORDERS" required:"true"`

	// Webhook processing mode: "sync" (direct) or "kafka" (async via Kafka)
	WebhookMode string `env:"WEBHOOK_MODE" envDefault:"sync"`

	// Kafka configuration
	KafkaBrokers               []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaOrdersTopic           string   `env:"KAFKA_ORDERS_TOPIC" envDefault:"webhooks.orders"`
	KafkaDisputesTopic         string   `env:"KAFKA_DISPUTES_TOPIC" envDefault:"webhooks.disputes"`
	KafkaOrdersConsumerGroup   string   `env:"KAFKA_ORDERS_CONSUMER_GROUP" envDefault:"payment-app-orders"`
	KafkaDisputesConsumerGroup string   `env:"KAFKA_DISPUTES_CONSUMER_GROUP" envDefault:"payment-app-disputes"`
}

func New() (Config, error) {
	c, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, err
	}

	return c, nil
}
