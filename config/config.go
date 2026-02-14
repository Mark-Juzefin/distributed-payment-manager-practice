package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// IngestConfig - configuration for Ingest service (HTTP → Kafka gateway)
type IngestConfig struct {
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

// CDCConfig - configuration for CDC worker (WAL → log/Kafka)
type CDCConfig struct {
	PgURL           string `env:"PG_URL" required:"true"`
	SlotName        string `env:"CDC_SLOT_NAME" envDefault:"cdc_slot"`
	PublicationName string `env:"CDC_PUBLICATION" envDefault:"events_pub"`
	LogLevel        string `env:"LOG_LEVEL" envDefault:"info"`

	KafkaBrokers     []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaEventsTopic string   `env:"KAFKA_EVENTS_TOPIC" envDefault:"domain.events"`
}

// AnalyticsConfig - configuration for Analytics consumer (Kafka → OpenSearch)
type AnalyticsConfig struct {
	LogLevel           string   `env:"LOG_LEVEL" envDefault:"info"`
	KafkaBrokers       []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaEventsTopic   string   `env:"KAFKA_EVENTS_TOPIC" envDefault:"domain.events"`
	KafkaConsumerGroup string   `env:"KAFKA_EVENTS_CONSUMER_GROUP" envDefault:"analytics-projection"`
	OpensearchURLs     []string `env:"OPENSEARCH_URLS" envSeparator:","`
	OpensearchIndex    string   `env:"OPENSEARCH_INDEX" envDefault:"domain-events"`
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

// NewCDCConfig parses environment variables for CDC worker
func NewCDCConfig() (CDCConfig, error) {
	c, err := env.ParseAs[CDCConfig]()
	if err != nil {
		return CDCConfig{}, err
	}

	return c, nil
}

// NewAnalyticsConfig parses environment variables for Analytics consumer
func NewAnalyticsConfig() (AnalyticsConfig, error) {
	c, err := env.ParseAs[AnalyticsConfig]()
	if err != nil {
		return AnalyticsConfig{}, err
	}

	return c, nil
}

// New - backward compatibility function, uses NewAPIConfig
func New() (Config, error) {
	return NewAPIConfig()
}
