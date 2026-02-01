package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	KafkaProcessingDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "dpm",
			Subsystem: "kafka",
			Name:      "message_processing_duration_seconds",
			Help:      "Kafka message processing duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"topic", "consumer_group", "status"},
	)

	KafkaMessagesProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "dpm",
			Subsystem: "kafka",
			Name:      "messages_processed_total",
			Help:      "Total number of Kafka messages processed",
		},
		[]string{"topic", "consumer_group", "status"},
	)

	// TODO: Add KafkaConsumerLag gauge when lag monitoring is needed.
	// Consumer lag shows how far behind the consumer is from the latest message.
	// Requires background goroutine to periodically poll kafka.Reader.Stats().Lag
)

func init() {
	Registry.MustRegister(KafkaProcessingDuration, KafkaMessagesProcessed)
}
