package api

import (
	"context"
	"log/slog"

	"TestTaskJustPay/pkg/kafka"
	"TestTaskJustPay/pkg/messaging"
	"TestTaskJustPay/services/paymanager/config"
	"TestTaskJustPay/services/paymanager/dispute"
	disputectrl "TestTaskJustPay/services/paymanager/dispute/controller"
	"TestTaskJustPay/services/paymanager/order"
	orderctrl "TestTaskJustPay/services/paymanager/order/controller"
	"TestTaskJustPay/services/paymanager/payment"
	paymentctrl "TestTaskJustPay/services/paymanager/payment/controller"
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

	orderController := orderctrl.NewKafkaHandler(orderService)
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

	disputeController := disputectrl.NewKafkaHandler(disputeService)
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

	paymentController := paymentctrl.NewKafkaHandler(paymentService)
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
