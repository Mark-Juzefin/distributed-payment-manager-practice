package api

import (
	"context"
	"log/slog"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/api/consumers"
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/external/kafka"
	"TestTaskJustPay/internal/api/messaging"
)

// StartWorkers starts Kafka consumers for order and dispute processing.
// It runs in a separate goroutine and will stop when ctx is cancelled.
func StartWorkers(
	ctx context.Context,
	cfg config.Config,
	orderService *order.OrderService,
	disputeService *dispute.DisputeService,
) {
	// Create DLQ publishers
	orderDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaOrdersDLQTopic)
	disputeDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaDisputesDLQTopic)
	defer orderDLQPub.Close()
	defer disputeDLQPub.Close()

	// Order consumer with metrics + retry + DLQ middleware
	orderController := consumers.NewOrderMessageController(orderService)
	orderHandler := messaging.WithMetrics(
		cfg.KafkaOrdersTopic,
		cfg.KafkaOrdersConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(orderController.HandleMessage, messaging.DefaultRetryConfig()),
			orderDLQPub,
		),
	)
	orderConsumer := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaOrdersTopic,
		cfg.KafkaOrdersConsumerGroup,
	)
	orderRunner := messaging.NewRunner([]messaging.Worker{orderConsumer}, orderHandler)

	// Dispute consumer with metrics + retry + DLQ middleware
	disputeController := consumers.NewDisputeMessageController(disputeService)
	disputeHandler := messaging.WithMetrics(
		cfg.KafkaDisputesTopic,
		cfg.KafkaDisputesConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(disputeController.HandleMessage, messaging.DefaultRetryConfig()),
			disputeDLQPub,
		),
	)
	disputeConsumer := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaDisputesTopic,
		cfg.KafkaDisputesConsumerGroup,
	)
	disputeRunner := messaging.NewRunner([]messaging.Worker{disputeConsumer}, disputeHandler)

	// Start order runner in background
	go func() {
		slog.Info("Starting order webhook consumer",
			"topic", cfg.KafkaOrdersTopic,
			"group", cfg.KafkaOrdersConsumerGroup)
		if err := orderRunner.Start(ctx); err != nil {
			slog.Error("Order runner failed", slog.Any("error", err))
		}
	}()

	// Start dispute runner in background
	go func() {
		slog.Info("Starting dispute webhook consumer",
			"topic", cfg.KafkaDisputesTopic,
			"group", cfg.KafkaDisputesConsumerGroup)
		if err := disputeRunner.Start(ctx); err != nil {
			slog.Error("Dispute runner failed", slog.Any("error", err))
		}
	}()
}
