package config

import "github.com/caarlos0/env/v11"

// Config holds CDC worker configuration.
type Config struct {
	PgURL           string `env:"PG_URL" required:"true"`
	SlotName        string `env:"CDC_SLOT_NAME" envDefault:"cdc_slot"`
	PublicationName string `env:"CDC_PUBLICATION" envDefault:"events_pub"`
	LogLevel        string `env:"LOG_LEVEL" envDefault:"info"`

	KafkaBrokers     []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaEventsTopic string   `env:"KAFKA_EVENTS_TOPIC" envDefault:"domain.events"`
}

// New parses environment variables for the CDC worker.
func New() (Config, error) {
	return env.ParseAs[Config]()
}
