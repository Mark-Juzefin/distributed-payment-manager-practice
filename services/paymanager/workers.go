package paymanager

import (
	"context"
	"log/slog"

	"TestTaskJustPay/pkg/kafka"
	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/config"
	"TestTaskJustPay/services/paymanager/internal/dispute"
	"TestTaskJustPay/services/paymanager/internal/dispute/disputecontroller"
	"TestTaskJustPay/services/paymanager/internal/order"
	"TestTaskJustPay/services/paymanager/internal/order/ordercontroller"
	"TestTaskJustPay/services/paymanager/internal/payment"
	"TestTaskJustPay/services/paymanager/internal/payment/paymentcontroller"
)

func StartWorkers(
	ctx context.Context,
	cfg config.Config,
	orderService *order.OrderService,
	disputeService *dispute.DisputeService,
	paymentService *payment.PaymentService,
) {
	orderDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaOrdersDLQTopic)
	disputeDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaDisputesDLQTopic)
	paymentDLQPub := kafka.NewDLQPublisher(cfg.KafkaBrokers, cfg.KafkaPaymentsDLQTopic)
	defer orderDLQPub.Close()
	defer disputeDLQPub.Close()
	defer paymentDLQPub.Close()

	orderController := ordercontroller.NewKafkaHandler(orderService)
	orderHandler := messaging.WithMetrics(
		cfg.KafkaOrdersTopic,
		cfg.KafkaOrdersConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(orderController.HandleMessage, messaging.DefaultRetryConfig()),
			orderDLQPub,
		),
	)
	orderRunner := messaging.NewRunner(
		[]messaging.Worker{kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaOrdersTopic, cfg.KafkaOrdersConsumerGroup)},
		orderHandler,
	)

	disputeController := disputecontroller.NewKafkaHandler(disputeService)
	disputeHandler := messaging.WithMetrics(
		cfg.KafkaDisputesTopic,
		cfg.KafkaDisputesConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(disputeController.HandleMessage, messaging.DefaultRetryConfig()),
			disputeDLQPub,
		),
	)
	disputeRunner := messaging.NewRunner(
		[]messaging.Worker{kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaDisputesTopic, cfg.KafkaDisputesConsumerGroup)},
		disputeHandler,
	)

	paymentController := paymentcontroller.NewKafkaHandler(paymentService)
	paymentHandler := messaging.WithMetrics(
		cfg.KafkaPaymentsTopic,
		cfg.KafkaPaymentsConsumerGroup,
		messaging.WithDLQ(
			messaging.WithRetry(paymentController.HandleMessage, messaging.DefaultRetryConfig()),
			paymentDLQPub,
		),
	)
	paymentRunner := messaging.NewRunner(
		[]messaging.Worker{kafka.NewConsumer(cfg.KafkaBrokers, cfg.KafkaPaymentsTopic, cfg.KafkaPaymentsConsumerGroup)},
		paymentHandler,
	)

	go func() {
		slog.Info("Starting order webhook consumer",
			"topic", cfg.KafkaOrdersTopic,
			"group", cfg.KafkaOrdersConsumerGroup)
		if err := orderRunner.Start(ctx); err != nil {
			slog.Error("Order runner failed", slog.Any("error", err))
		}
	}()

	go func() {
		slog.Info("Starting dispute webhook consumer",
			"topic", cfg.KafkaDisputesTopic,
			"group", cfg.KafkaDisputesConsumerGroup)
		if err := disputeRunner.Start(ctx); err != nil {
			slog.Error("Dispute runner failed", slog.Any("error", err))
		}
	}()

	go func() {
		slog.Info("Starting payment webhook consumer",
			"topic", cfg.KafkaPaymentsTopic,
			"group", cfg.KafkaPaymentsConsumerGroup)
		if err := paymentRunner.Start(ctx); err != nil {
			slog.Error("Payment runner failed", slog.Any("error", err))
		}
	}()
}
