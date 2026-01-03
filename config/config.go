package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// IngestConfig - minimal configuration for Ingest service (HTTP → Kafka gateway)
type IngestConfig struct {
	Port     int    `env:"PORT" envDefault:"3001"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info"`

	// Kafka (required for Ingest service)
	KafkaBrokers       []string `env:"KAFKA_BROKERS" envSeparator:"," required:"true"`
	KafkaOrdersTopic   string   `env:"KAFKA_ORDERS_TOPIC" envDefault:"webhooks.orders"`
	KafkaDisputesTopic string   `env:"KAFKA_DISPUTES_TOPIC" envDefault:"webhooks.disputes"`
}

// APIConfig - full configuration for API service (domain logic, consumers, manual operations)
type APIConfig struct {
	Port      int    `env:"PORT" envDefault:"3000"`
	PgURL     string `env:"PG_URL" required:"true"`
	PgPoolMax int    `env:"PG_POOL_MAX" envDefault:"10"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`

	SilvergateBaseURL                 string        `env:"SILVERGATE_BASE_URL" required:"true"`
	SilvergateSubmitRepresentmentPath string        `env:"SILVERGATE_SUBMIT_REPRESENTMENT_PATH" required:"true"`
	SilvergateCapturePath             string        `env:"SILVERGATE_CAPTURE_PATH" required:"true"`
	HTTPSilvergateClientTimeout       time.Duration `env:"HTTP_SILVERGATE_CLIENT_TIMEOUT" envDefault:"20s"`

	// OpenSearch is optional
	OpensearchUrls          []string `env:"OPENSEARCH_URLS" envSeparator:","`
	OpensearchIndexDisputes string   `env:"OPENSEARCH_INDEX_DISPUTES" envDefault:"events-disputes"`
	OpensearchIndexOrders   string   `env:"OPENSEARCH_INDEX_ORDERS" envDefault:"events-orders"`

	// Webhook processing mode: "sync" (direct) or "kafka" (async via Kafka)
	WebhookMode string `env:"WEBHOOK_MODE" envDefault:"sync"`

	// Kafka configuration (required only if WebhookMode=kafka)
	KafkaBrokers               []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaOrdersTopic           string   `env:"KAFKA_ORDERS_TOPIC" envDefault:"webhooks.orders"`
	KafkaDisputesTopic         string   `env:"KAFKA_DISPUTES_TOPIC" envDefault:"webhooks.disputes"`
	KafkaOrdersConsumerGroup   string   `env:"KAFKA_ORDERS_CONSUMER_GROUP" envDefault:"payment-app-orders"`
	KafkaDisputesConsumerGroup string   `env:"KAFKA_DISPUTES_CONSUMER_GROUP" envDefault:"payment-app-disputes"`
	KafkaOrdersDLQTopic        string   `env:"KAFKA_ORDERS_DLQ_TOPIC" envDefault:"webhooks.orders.dlq"`
	KafkaDisputesDLQTopic      string   `env:"KAFKA_DISPUTES_DLQ_TOPIC" envDefault:"webhooks.disputes.dlq"`
}

// Config - backward compatibility alias for APIConfig
type Config = APIConfig

// NewIngestConfig parses environment variables for Ingest service
func NewIngestConfig() (IngestConfig, error) {
	c, err := env.ParseAs[IngestConfig]()
	if err != nil {
		return IngestConfig{}, err
	}

	return c, nil
}

// NewAPIConfig parses environment variables for API service
func NewAPIConfig() (APIConfig, error) {
	c, err := env.ParseAs[APIConfig]()
	if err != nil {
		return APIConfig{}, err
	}

	return c, nil
}

// New - backward compatibility function, uses NewAPIConfig
func New() (Config, error) {
	return NewAPIConfig()
}
