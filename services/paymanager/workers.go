package api

import (
	"context"
	"log/slog"

	"TestTaskJustPay/pkg/kafka"
	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/config"
	"TestTaskJustPay/services/paymanager/consumers"
	"TestTaskJustPay/services/paymanager/domain/dispute"
	"TestTaskJustPay/services/paymanager/domain/order"
	"TestTaskJustPay/services/paymanager/domain/payment"
)

// StartWorkers starts Kafka consumers for order and dispute processing.
// It runs in a separate goroutine and will stop when ctx is cancelled.
func StartWorkers(
	ctx context.Context,
	cfg config.Config,
	orderService *order.OrderService,
	disputeService *dispute.DisputeService,
	paymentService *payment.PaymentService,
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

	// Payment consumer with metrics + retry + DLQ middleware
	paymentDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaPaymentsDLQTopic)
	defer paymentDLQPub.Close()

	paymentController := consumers.NewPaymentMessageController(paymentService)
	paymentHandler := messaging.WithMetrics(
		cfg.KafkaPaymentsTopic,
		cfg.KafkaPaymentsConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(paymentController.HandleMessage, messaging.DefaultRetryConfig()),
			paymentDLQPub,
		),
	)
	paymentConsumer := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaPaymentsTopic,
		cfg.KafkaPaymentsConsumerGroup,
	)
	paymentRunner := messaging.NewRunner([]messaging.Worker{paymentConsumer}, paymentHandler)

	go func() {
		slog.Info("Starting payment webhook consumer",
			"topic", cfg.KafkaPaymentsTopic,
			"group", cfg.KafkaPaymentsConsumerGroup)
		if err := paymentRunner.Start(ctx); err != nil {
			slog.Error("Payment runner failed", slog.Any("error", err))
		}
	}()
}
