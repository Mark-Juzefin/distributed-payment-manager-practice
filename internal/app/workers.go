package app

import (
	"context"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/message"
	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/domain/order"
	"TestTaskJustPay/internal/shared/external/kafka"
	"TestTaskJustPay/internal/shared/messaging"
	"TestTaskJustPay/pkg/logger"
)

// StartWorkers starts Kafka consumers for order and dispute processing.
// It runs in a separate goroutine and will stop when ctx is cancelled.
func StartWorkers(
	ctx context.Context,
	l *logger.Logger,
	cfg config.Config,
	orderService *order.OrderService,
	disputeService *dispute.DisputeService,
) {
	// Create DLQ publishers
	orderDLQPub := kafka.NewDLQPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersDLQTopic)
	disputeDLQPub := kafka.NewDLQPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesDLQTopic)
	defer orderDLQPub.Close()
	defer disputeDLQPub.Close()

	// Order consumer with retry + DLQ middleware
	orderController := message.NewOrderMessageController(l, orderService)
	orderHandler := messaging.WithDLQ(
		messaging.WithRetry(orderController.HandleMessage, messaging.DefaultRetryConfig()),
		orderDLQPub,
	)
	orderConsumer := kafka.NewConsumer(
		l,
		cfg.KafkaBrokers,
		cfg.KafkaOrdersTopic,
		cfg.KafkaOrdersConsumerGroup,
	)
	orderRunner := messaging.NewRunner(l, []messaging.Worker{orderConsumer}, orderHandler)

	// Dispute consumer with retry + DLQ middleware
	disputeController := message.NewDisputeMessageController(l, disputeService)
	disputeHandler := messaging.WithDLQ(
		messaging.WithRetry(disputeController.HandleMessage, messaging.DefaultRetryConfig()),
		disputeDLQPub,
	)
	disputeConsumer := kafka.NewConsumer(
		l,
		cfg.KafkaBrokers,
		cfg.KafkaDisputesTopic,
		cfg.KafkaDisputesConsumerGroup,
	)
	disputeRunner := messaging.NewRunner(l, []messaging.Worker{disputeConsumer}, disputeHandler)

	// Start order runner in background
	go func() {
		l.Info("Starting order webhook consumer: topic=%s group=%s",
			cfg.KafkaOrdersTopic, cfg.KafkaOrdersConsumerGroup)
		if err := orderRunner.Start(ctx); err != nil {
			l.Error("Order runner failed: error=%v", err)
		}
	}()

	// Start dispute runner in background
	go func() {
		l.Info("Starting dispute webhook consumer: topic=%s group=%s",
			cfg.KafkaDisputesTopic, cfg.KafkaDisputesConsumerGroup)
		if err := disputeRunner.Start(ctx); err != nil {
			l.Error("Dispute runner failed: error=%v", err)
		}
	}()
}
