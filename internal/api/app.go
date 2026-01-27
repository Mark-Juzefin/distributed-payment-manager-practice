package api

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/external/silvergate"
	"TestTaskJustPay/internal/api/handlers"
	"TestTaskJustPay/internal/api/handlers/updates"
	dispute_repo "TestTaskJustPay/internal/api/repo/dispute"
	"TestTaskJustPay/internal/api/repo/dispute_eventsink"
	order_repo "TestTaskJustPay/internal/api/repo/order"
	"TestTaskJustPay/internal/api/repo/order_eventsink"
	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
)

//go:embed migrations/*.sql
var MIGRATION_FS embed.FS

func Run(cfg config.Config) {
	logger.Setup(logger.Options{
		Level:   cfg.LogLevel,
		Console: strings.ToLower(os.Getenv("LOG_FORMAT")) == "console",
	})

	// Setup graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	engine := NewGinEngine()

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		slog.Error("Failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
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
	orderService := order.NewOrderService(orderRepo, silvergateClient, orderEventSink)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, disputeEventSink)

	// Handlers (clean - no processor dependency)
	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	// Internal handlers (for service-to-service communication)
	updatesHandler := updates.NewUpdatesHandler(orderService, disputeService)

	// Health checks registry
	var healthCheckers []health.Checker
	healthCheckers = append(healthCheckers, health.NewPostgresChecker(pool.Pool))
	if cfg.WebhookMode == "kafka" {
		healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
	}
	healthRegistry := health.NewRegistry(healthCheckers...)

	// Router (API endpoints only)
	router := NewRouter(orderHandler, chargebackHandler, disputeHandler, healthRegistry)
	router.SetUp(engine)

	// Internal router (service-to-service endpoints)
	internalRouter := NewInternalRouter(updatesHandler)
	internalRouter.SetUp(engine)

	// Apply migrations
	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		slog.Error("Failed to apply migrations", slog.Any("error", err))
		os.Exit(1)
	}

	// Start Kafka consumers if in kafka mode
	if cfg.WebhookMode == "kafka" {
		slog.Info("Webhook mode: kafka - starting Kafka consumers")
		StartWorkers(ctx, cfg, orderService, disputeService)
	}

	// Start HTTP server in a goroutine
	go func() {
		slog.Info("Starting API HTTP server", "port", cfg.Port)
		if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("HTTP server error", slog.Any("error", err))
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("Shutting down API service gracefully...")
}
