package health

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// KafkaChecker checks Kafka broker connectivity.
type KafkaChecker struct {
	brokers []string
}

// NewKafkaChecker creates a new Kafka health checker.
func NewKafkaChecker(brokers []string) *KafkaChecker {
	return &KafkaChecker{brokers: brokers}
}

// Name returns "kafka".
func (c *KafkaChecker) Name() string {
	return "kafka"
}

// Check attempts to connect to any Kafka broker.
func (c *KafkaChecker) Check(ctx context.Context) Result {
	for _, broker := range c.brokers {
		conn, err := kafka.DialContext(ctx, "tcp", broker)
		if err == nil {
			_ = conn.Close()
			return Result{Status: StatusUp}
		}
	}
	return Result{Status: StatusDown, Message: "all brokers unreachable"}
}
