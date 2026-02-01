package kafka

import (
	"context"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"
)

// DLQPublisher publishes failed messages to a Dead Letter Queue topic.
type DLQPublisher struct {
	writer *kafka.Writer
}

// NewDLQPublisher creates a new DLQ publisher.
func NewDLQPublisher(brokers []string, dlqTopic string) *DLQPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
	}

	return &DLQPublisher{
		writer: writer,
	}
}

// PublishToDLQ sends a failed message to DLQ with error information in headers.
func (p *DLQPublisher) PublishToDLQ(ctx context.Context, key, value []byte, err error) error {
	msg := kafka.Message{
		Key:   key,
		Value: value,
		Headers: []kafka.Header{
			{Key: "error", Value: []byte(err.Error())},
			{Key: "failed_at", Value: []byte(time.Now().UTC().Format(time.RFC3339))},
		},
	}

	if writeErr := p.writer.WriteMessages(ctx, msg); writeErr != nil {
		slog.Error("Failed to publish to DLQ",
			"topic", p.writer.Topic,
			"key", string(key),
			slog.Any("error", writeErr),
			slog.Any("original_error", err))
		return writeErr
	}

	slog.Warn("Message sent to DLQ",
		"topic", p.writer.Topic,
		"key", string(key),
		slog.Any("error", err))
	return nil
}

// Close closes the DLQ writer.
func (p *DLQPublisher) Close() error {
	return p.writer.Close()
}
