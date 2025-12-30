package kafka

import (
	"context"
	"errors"

	"TestTaskJustPay/internal/messaging"
	"TestTaskJustPay/pkg/logger"

	"github.com/segmentio/kafka-go"
)

// Consumer implements messaging.Worker using Kafka.
type Consumer struct {
	reader *kafka.Reader
	logger *logger.Logger
}

// NewConsumer creates a new Kafka consumer.
func NewConsumer(l *logger.Logger, brokers []string, topic, groupID string) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
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
			if errors.Is(err, context.Canceled) {
				c.logger.Info("Consumer stopped (context cancelled)")
				return nil
			}
			c.logger.Error("Failed to fetch message: error=%v", err)
			return err
		}

		c.logger.Debug("Message received: topic=%s partition=%d offset=%d key=%s",
			msg.Topic, msg.Partition, msg.Offset, string(msg.Key))

		if err := handler(ctx, msg.Key, msg.Value); err != nil {
			c.logger.Error("Handler error, message not committed: topic=%s partition=%d offset=%d key=%s error=%v",
				msg.Topic, msg.Partition, msg.Offset, string(msg.Key), err)
			// Don't commit - message will be redelivered on restart
			continue
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("Failed to commit message: topic=%s partition=%d offset=%d error=%v",
				msg.Topic, msg.Partition, msg.Offset, err)
			return err
		}

		c.logger.Debug("Message committed: topic=%s partition=%d offset=%d",
			msg.Topic, msg.Partition, msg.Offset)
	}
}

// Close closes the Kafka reader.
func (c *Consumer) Close() error {
	c.logger.Info("Closing consumer: topic=%s group_id=%s",
		c.reader.Config().Topic, c.reader.Config().GroupID)
	return c.reader.Close()
}
