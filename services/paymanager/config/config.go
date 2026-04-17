package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds API service configuration.
type Config struct {
	Port         int    `env:"PORT" envDefault:"3000"`
	PgURL        string `env:"PG_URL" required:"true"`
	PgReplicaURL string `env:"PG_REPLICA_URL"`
	PgPoolMax    int    `env:"PG_POOL_MAX" envDefault:"10"`
	LogLevel     string `env:"LOG_LEVEL" envDefault:"info"`

	SilvergateBaseURL                 string        `env:"SILVERGATE_BASE_URL" required:"true"`
	SilvergateSubmitRepresentmentPath string        `env:"SILVERGATE_SUBMIT_REPRESENTMENT_PATH" required:"true"`
	SilvergateCapturePath             string        `env:"SILVERGATE_CAPTURE_PATH" required:"true"`
	SilvergateAuthPath                string        `env:"SILVERGATE_AUTH_PATH" envDefault:"/api/v1/auth"`
	SilvergateVoidPath                string        `env:"SILVERGATE_VOID_PATH" envDefault:"/api/v1/void"`
	HTTPSilvergateClientTimeout       time.Duration `env:"HTTP_SILVERGATE_CLIENT_TIMEOUT" envDefault:"20s"`

	MerchantID string `env:"MERCHANT_ID" envDefault:"merchant_1"`

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
	KafkaPaymentsTopic         string   `env:"KAFKA_PAYMENTS_TOPIC" envDefault:"webhooks.payments"`
	KafkaPaymentsConsumerGroup string   `env:"KAFKA_PAYMENTS_CONSUMER_GROUP" envDefault:"payment-app-payments"`
	KafkaPaymentsDLQTopic      string   `env:"KAFKA_PAYMENTS_DLQ_TOPIC" envDefault:"webhooks.payments.dlq"`
}

// New parses environment variables for the API service.
func New() (Config, error) {
	return env.ParseAs[Config]()
}
