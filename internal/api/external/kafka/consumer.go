package kafka

import (
	"context"
	"errors"
	"time"

	"TestTaskJustPay/internal/api/messaging"
	"TestTaskJustPay/pkg/correlation"
	"TestTaskJustPay/pkg/logger"

	"github.com/segmentio/kafka-go"
)

const (
	commitTimeout = 5 * time.Second
)

// Consumer implements messaging.Worker using Kafka.
type Consumer struct {
	reader *kafka.Reader
	logger *logger.Logger
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(l *logger.Logger, brokers []string, topic, groupID string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		CommitInterval: 0,    // Commit synchronously for reliability
		StartOffset:    kafka.FirstOffset,
		// Faster consumer group coordination
		MaxWait:          500 * time.Millisecond,
		RebalanceTimeout: 5 * time.Second,
	})

	return &Consumer{
		reader: reader,
		logger: l,
	}
}

// Start begins consuming messages and passes them to the handler.
// Blocks until context is cancelled or an unrecoverable error occurs.
func (c *Consumer) Start(ctx context.Context, handler messaging.MessageHandler) error {
	c.logger.Info("Consumer started: topic=%s group_id=%s",
		c.reader.Config().Topic, c.reader.Config().GroupID)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			// Graceful shutdown - context cancellation is not an error
			if errors.Is(err, context.Canceled) {
				c.logger.Info("Consumer stopped (context cancelled)")
				return nil
			}
			c.logger.Error("Failed to fetch message: error=%v", err)
			return err
		}

		// Extract correlation ID from Kafka headers and inject into context
		msgCtx := extractCorrelationID(ctx, msg.Headers)

		c.logger.DebugCtx(msgCtx, "Message received: topic=%s partition=%d offset=%d key=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key))

		if err := handler(msgCtx, msg.Key, msg.Value); err != nil {
			c.logger.ErrorCtx(msgCtx, "Handler error, message not committed: topic=%s partition=%d offset=%d key=%s error=%v",
				msg.Topic, msg.Partition, msg.Offset, string(msg.Key), err)
			// Don't commit - message will be redelivered on restart
			continue
		}

		// Use separate context for commit to avoid losing successfully processed messages
		// when main context is cancelled during shutdown
		commitCtx, cancel := context.WithTimeout(context.Background(), commitTimeout)
		err = c.reader.CommitMessages(commitCtx, msg)
		cancel()
		if err != nil {
			c.logger.ErrorCtx(msgCtx, "Failed to commit message: topic=%s partition=%d offset=%d error=%v",
				msg.Topic, msg.Partition, msg.Offset, err)
			// Don't return error - message was processed, commit failure is not critical
			// It will be redelivered on restart but idempotency will handle it
		}

		c.logger.DebugCtx(msgCtx, "Message committed: topic=%s partition=%d offset=%d",
			msg.Topic, msg.Partition, msg.Offset)
	}
}

// Close closes the Kafka reader.
func (c *Consumer) Close() error {
	c.logger.Info("Closing consumer: topic=%s group_id=%s",
		c.reader.Config().Topic, c.reader.Config().GroupID)
	return c.reader.Close()
}

// extractCorrelationID extracts correlation ID from Kafka headers and returns enriched context.
// If no correlation ID header found, generates a new one.
func extractCorrelationID(ctx context.Context, headers []kafka.Header) context.Context {
	for _, h := range headers {
		if h.Key == correlation.KafkaHeaderName {
			return correlation.WithID(ctx, string(h.Value))
		}
	}
	// Generate new ID if not present (for messages published before this feature)
	return correlation.WithID(ctx, correlation.NewID())
}
