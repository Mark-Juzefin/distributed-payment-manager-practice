package app

import (
	"context"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/message"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/external/kafka"
	"TestTaskJustPay/internal/messaging"
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
	// Order consumer
	orderController := message.NewOrderMessageController(l, orderService)
	orderConsumer := kafka.NewConsumer(
		l,
		cfg.KafkaBrokers,
		cfg.KafkaOrdersTopic,
		cfg.KafkaOrdersConsumerGroup,
	)
	orderRunner := messaging.NewRunner(l, []messaging.Worker{orderConsumer}, orderController.HandleMessage)

	// Dispute consumer
	disputeController := message.NewDisputeMessageController(l, disputeService)
	disputeConsumer := kafka.NewConsumer(
		l,
		cfg.KafkaBrokers,
		cfg.KafkaDisputesTopic,
		cfg.KafkaDisputesConsumerGroup,
	)
	disputeRunner := messaging.NewRunner(l, []messaging.Worker{disputeConsumer}, disputeController.HandleMessage)

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
