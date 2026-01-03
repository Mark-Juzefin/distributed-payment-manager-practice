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
	"TestTaskJustPay/internal/ingest/handlers"
	"TestTaskJustPay/internal/api/external/kafka"
	"TestTaskJustPay/internal/ingest/webhook"
	"TestTaskJustPay/pkg/logger"

	"github.com/gin-gonic/gin"
)

// Run bootstraps and runs the Ingest service
func Run(cfg config.IngestConfig) {
	l := logger.New(cfg.LogLevel)

	// TODO(Subtask 2): Implement sync mode with gRPC call to API service
	// For now, only kafka mode is supported
	if cfg.WebhookMode != "kafka" {
		l.Fatal(fmt.Errorf("ingest - Run - unsupported webhook mode: %s (only 'kafka' is supported, 'sync' will be implemented via gRPC in Subtask 2)", cfg.WebhookMode))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Setup Gin engine
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// Kafka publishers (kafka mode)
	l.Info("Webhook mode: kafka - initializing Kafka publishers")
	l.Info("Kafka publishers: brokers=%v, ordersTopic=%s, disputesTopic=%s",
		cfg.KafkaBrokers, cfg.KafkaOrdersTopic, cfg.KafkaDisputesTopic)
	orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
	disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
	defer func() { _ = orderPublisher.Close() }()
	defer func() { _ = disputePublisher.Close() }()

	// AsyncProcessor - publishes webhooks to Kafka
	processor := webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

	// Handlers (processor only, clean)
	orderHandler := handlers.NewOrderHandler(processor)
	chargebackHandler := handlers.NewChargebackHandler(processor)

	// Webhook-only routes
	router := NewRouter(orderHandler, chargebackHandler)
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
