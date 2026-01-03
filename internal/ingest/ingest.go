package ingest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/app"
	"TestTaskJustPay/internal/controller/rest"
	"TestTaskJustPay/internal/controller/rest/handlers"
	"TestTaskJustPay/internal/shared/external/kafka"
	"TestTaskJustPay/internal/shared/webhook"
	"TestTaskJustPay/pkg/logger"
)

// Run bootstraps and runs the Ingest service (lightweight HTTP → Kafka gateway)
func Run(cfg config.IngestConfig) {
	l := logger.New(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	engine := app.NewGinEngine(l)

	// Kafka publishers
	l.Info("Initializing Kafka publishers: brokers=%v, ordersTopic=%s, disputesTopic=%s",
		cfg.KafkaBrokers, cfg.KafkaOrdersTopic, cfg.KafkaDisputesTopic)
	orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
	disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
	defer func() { _ = orderPublisher.Close() }()
	defer func() { _ = disputePublisher.Close() }()

	// AsyncProcessor - publishes webhooks to Kafka
	processor := webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

	// Handlers (service=nil for Ingest mode - no business logic here)
	orderHandler := handlers.NewOrderHandler(nil, processor)
	chargebackHandler := handlers.NewChargebackHandler(nil, processor)

	// Webhook-only routes
	router := rest.NewWebhookRouter(orderHandler, chargebackHandler)
	router.SetUp(engine)

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: engine,
	}

	go func() {
		l.Info("Ingest service started: port=%d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	l.Info("Shutting down Ingest service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		l.Error("Server shutdown error: %v", err)
	}

	l.Info("Ingest service stopped")
}
