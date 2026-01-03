package api

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/api/handlers"
	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/domain/order"
	"TestTaskJustPay/internal/shared/external/silvergate"
	dispute_repo "TestTaskJustPay/internal/shared/repo/dispute"
	"TestTaskJustPay/internal/shared/repo/dispute_eventsink"
	order_repo "TestTaskJustPay/internal/shared/repo/order"
	"TestTaskJustPay/internal/shared/repo/order_eventsink"
	"TestTaskJustPay/internal/shared/webhook"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"

	ingestHandlers "TestTaskJustPay/internal/ingest/handlers"
)

//go:embed migrations/*.sql
var MIGRATION_FS embed.FS

func Run(cfg config.Config) {
	l := logger.New(cfg.LogLevel)

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	engine := NewGinEngine(l)

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("api - Run - postgres.NewPgPool: %w", err))
	}
	defer pool.Close()

	orderRepo := order_repo.NewPgOrderRepo(pool)
	disputeRepo := dispute_repo.NewPgDisputeRepo(pool)
	disputeEventSink := dispute_eventsink.NewPgEventRepo(pool.Pool, pool.Builder)
	orderEventSink := order_eventsink.NewPgOrderEventRepo(pool.Pool, pool.Builder)

	silvergateClient := silvergate.New(
		cfg.SilvergateBaseURL,
		cfg.SilvergateSubmitRepresentmentPath,
		cfg.SilvergateCapturePath,
		&http.Client{Timeout: cfg.HTTPSilvergateClientTimeout},
	)

	// Services
	orderService := order.NewOrderService(orderRepo, silvergateClient, orderEventSink, l)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, disputeEventSink, l)

	// Handlers (clean - no processor dependency)
	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	// Router (API endpoints only)
	router := NewRouter(orderHandler, chargebackHandler, disputeHandler)
	router.SetUp(engine)

	// Apply migrations
	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("api - Run - ApplyMigrations: %w", err))
	}

	// Mode-specific setup
	if cfg.WebhookMode == "kafka" {
		l.Info("Webhook mode: kafka - starting Kafka consumers")
		// Start Kafka consumers to process messages from Ingest service
		StartWorkers(ctx, l, cfg, orderService, disputeService)
	} else {
		l.Info("Webhook mode: sync - enabling webhook endpoints")
		// In sync mode, add webhook endpoints directly to API service
		processor := webhook.NewSyncProcessor(orderService, disputeService)
		webhookOrderHandler := ingestHandlers.NewOrderHandler(processor)
		webhookChargebackHandler := ingestHandlers.NewChargebackHandler(processor)

		// Add webhook routes
		engine.POST("/webhooks/payments/orders", webhookOrderHandler.Webhook)
		engine.POST("/webhooks/payments/chargebacks", webhookChargebackHandler.Webhook)
	}

	// Start HTTP server in a goroutine
	go func() {
		l.Info("Starting API HTTP server: port=%d", cfg.Port)
		if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			l.Error("HTTP server error: error=%v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	l.Info("Shutting down API service gracefully...")
}
