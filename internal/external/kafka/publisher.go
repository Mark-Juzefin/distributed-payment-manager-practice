package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"TestTaskJustPay/internal/messaging"
	"TestTaskJustPay/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// Publisher implements messaging.Publisher using Kafka.
type Publisher struct {
	writer *kafka.Writer
	logger *logger.Logger
}

// NewPublisher creates a new Kafka publisher.
func NewPublisher(l *logger.Logger, brokers []string, topic string) *Publisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
	}

	return &Publisher{
		writer: writer,
		logger: l,
	}
}

// Publish sends an envelope to Kafka with retry for transient errors.
func (p *Publisher) Publish(ctx context.Context, env messaging.Envelope) error {
	value, err := json.Marshal(env)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Key:   []byte(env.Key),
		Value: value,
	}

	// Retry with backoff for transient errors (Kafka may need time to start)
	const maxRetries = 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*2) * time.Second // 2s, 4s, 6s, 8s
			time.Sleep(backoff)
			p.logger.Info("Retrying publish: topic=%s attempt=%d backoff=%v", p.writer.Topic, attempt+1, backoff)
		}

		if err := p.writer.WriteMessages(ctx, msg); err != nil {
			lastErr = err
			if isRetryable(err) {
				continue
			}
			p.logger.Error("Failed to publish message: topic=%s key=%s error=%v",
				p.writer.Topic, env.Key, err)
			return err
		}

		p.logger.Debug("Message published: topic=%s key=%s event_id=%s",
			p.writer.Topic, env.Key, env.EventID)
		return nil
	}

	p.logger.Error("Failed to publish after retries: topic=%s key=%s error=%v",
		p.writer.Topic, env.Key, lastErr)
	return lastErr
}

func isRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "Unknown Topic Or Partition") ||
		strings.Contains(msg, "connection refused")
}

// Close closes the Kafka writer.
func (p *Publisher) Close() error {
	return p.writer.Close()
}
