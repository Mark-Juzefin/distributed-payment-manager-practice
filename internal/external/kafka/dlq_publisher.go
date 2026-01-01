package kafka

import (
	"context"
	"time"

	"TestTaskJustPay/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// DLQPublisher publishes failed messages to a Dead Letter Queue topic.
type DLQPublisher struct {
	writer *kafka.Writer
	logger *logger.Logger
}

// NewDLQPublisher creates a new DLQ publisher.
func NewDLQPublisher(l *logger.Logger, brokers []string, dlqTopic string) *DLQPublisher {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
	}

	return &DLQPublisher{
		writer: writer,
		logger: l,
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
		p.logger.Error("Failed to publish to DLQ: topic=%s key=%s error=%v original_error=%v",
			p.writer.Topic, string(key), writeErr, err)
		return writeErr
	}

	p.logger.Warn("Message sent to DLQ: topic=%s key=%s error=%v",
		p.writer.Topic, string(key), err)
	return nil
}

// Close closes the DLQ writer.
func (p *DLQPublisher) Close() error {
	return p.writer.Close()
}
