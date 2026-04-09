package config

import "github.com/caarlos0/env/v11"

// Config holds Analytics consumer configuration.
type Config struct {
	LogLevel           string   `env:"LOG_LEVEL" envDefault:"info"`
	KafkaBrokers       []string `env:"KAFKA_BROKERS" envSeparator:","`
	KafkaEventsTopic   string   `env:"KAFKA_EVENTS_TOPIC" envDefault:"domain.events"`
	KafkaConsumerGroup string   `env:"KAFKA_EVENTS_CONSUMER_GROUP" envDefault:"analytics-projection"`
	OpensearchURLs     []string `env:"OPENSEARCH_URLS" envSeparator:","`
	OpensearchIndex    string   `env:"OPENSEARCH_INDEX" envDefault:"domain-events"`
}

// New parses environment variables for the Analytics consumer.
func New() (Config, error) {
	return env.ParseAs[Config]()
}
