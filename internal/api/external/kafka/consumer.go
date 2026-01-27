package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"TestTaskJustPay/internal/api/messaging"
	"TestTaskJustPay/pkg/correlation"

	"github.com/segmentio/kafka-go"
)

const (
	commitTimeout = 5 * time.Second
)

// Consumer implements messaging.Worker using Kafka.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(brokers []string, topic, groupID string) *Consumer {
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
	}
}

// Start begins consuming messages and passes them to the handler.
// Blocks until context is cancelled or an unrecoverable error occurs.
func (c *Consumer) Start(ctx context.Context, handler messaging.MessageHandler) error {
	slog.Info("Consumer started",
		"topic", c.reader.Config().Topic,
		"group_id", c.reader.Config().GroupID)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			// Graceful shutdown - context cancellation is not an error
			if errors.Is(err, context.Canceled) {
				slog.Info("Consumer stopped (context cancelled)")
				return nil
			}
			slog.Error("Failed to fetch message", slog.Any("error", err))
			return err
		}

		// Extract correlation ID from Kafka headers and inject into context
		msgCtx := extractCorrelationID(ctx, msg.Headers)

		slog.DebugContext(msgCtx, "Message received",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset,
			"key", string(msg.Key))

		if err := handler(msgCtx, msg.Key, msg.Value); err != nil {
			slog.ErrorContext(msgCtx, "Handler error, message not committed",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				"key", string(msg.Key),
				slog.Any("error", err))
			// Don't commit - message will be redelivered on restart
			continue
		}

		// Use separate context for commit to avoid losing successfully processed messages
		// when main context is cancelled during shutdown
		commitCtx, cancel := context.WithTimeout(context.Background(), commitTimeout)
		err = c.reader.CommitMessages(commitCtx, msg)
		cancel()
		if err != nil {
			slog.ErrorContext(msgCtx, "Failed to commit message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
				slog.Any("error", err))
			// Don't return error - message was processed, commit failure is not critical
			// It will be redelivered on restart but idempotency will handle it
		}

		slog.DebugContext(msgCtx, "Message committed",
			"topic", msg.Topic,
			"partition", msg.Partition,
			"offset", msg.Offset)
	}
}

// Close closes the Kafka reader.
func (c *Consumer) Close() error {
	slog.Info("Closing consumer",
		"topic", c.reader.Config().Topic,
		"group_id", c.reader.Config().GroupID)
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
