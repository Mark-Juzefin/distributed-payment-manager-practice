package kafka

import (
	"context"
	"encoding/json"

	"TestTaskJustPay/internal/api/messaging"
	"TestTaskJustPay/pkg/correlation"
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

	// Add correlation ID header if present in context
	if corrID := correlation.FromContext(ctx); corrID != "" {
		msg.Headers = append(msg.Headers, kafka.Header{
			Key:   correlation.KafkaHeaderName,
			Value: []byte(corrID),
		})
	}

	if err = p.writer.WriteMessages(ctx, msg); err != nil {
		p.logger.Error("Failed to publish message: topic=%s key=%s error=%v",
			p.writer.Topic, env.Key, err)
		return err
	}

	p.logger.DebugCtx(ctx, "Message published: topic=%s key=%s event_id=%s",
		p.writer.Topic, env.Key, env.EventID)
	return nil

}

// Close closes the Kafka writer.
func (p *Publisher) Close() error {
	return p.writer.Close()
}
