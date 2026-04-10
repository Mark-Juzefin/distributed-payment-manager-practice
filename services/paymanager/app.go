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

	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/paymanager/config"
	"TestTaskJustPay/services/paymanager/domain/dispute"
	"TestTaskJustPay/services/paymanager/domain/order"
	"TestTaskJustPay/services/paymanager/domain/payment"
	"TestTaskJustPay/services/paymanager/external/silvergate"
	"TestTaskJustPay/services/paymanager/handlers"
	"TestTaskJustPay/services/paymanager/handlers/updates"
	dispute_repo "TestTaskJustPay/services/paymanager/repo/dispute"
	"TestTaskJustPay/services/paymanager/repo/dispute_eventsink"
	events_repo "TestTaskJustPay/services/paymanager/repo/events"
	order_repo "TestTaskJustPay/services/paymanager/repo/order"
	"TestTaskJustPay/services/paymanager/repo/order_eventsink"
	payment_repo "TestTaskJustPay/services/paymanager/repo/payment"
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

	// Read pool: replica if configured, otherwise same as primary
	var readDB postgres.Executor = pool.Pool
	var readPG *postgres.Postgres
	if cfg.PgReplicaURL != "" {
		readPG, err = postgres.New(cfg.PgReplicaURL, postgres.MaxPoolSize(cfg.PgPoolMax))
		if err != nil {
			slog.Error("Failed to connect to read replica", slog.Any("error", err))
			os.Exit(1)
		}
		defer readPG.Close()
		readDB = readPG.Pool
		slog.Info("Read replica pool configured")
	}

	orderRepo := order_repo.NewPgOrderRepo(pool, readDB)
	disputeRepo := dispute_repo.NewPgDisputeRepo(pool, readDB)
	disputeEvents := dispute_eventsink.NewPgEventRepo(pool.Pool, readDB, pool.Builder)
	orderEvents := order_eventsink.NewPgOrderEventRepo(pool.Pool, readDB, pool.Builder)

	silvergateClient := silvergate.New(
		cfg.SilvergateBaseURL,
		cfg.SilvergateSubmitRepresentmentPath,
		cfg.SilvergateCapturePath,
		cfg.SilvergateAuthPath,
		cfg.SilvergateVoidPath,
		&http.Client{Timeout: cfg.HTTPSilvergateClientTimeout},
	)

	// Services
	eventStoreFactory := events_repo.TxStoreFactory(pool.Builder)
	orderService := order.NewOrderService(pool, order_repo.TxRepoFactory(pool.Builder), eventStoreFactory, orderRepo, silvergateClient, orderEvents)
	disputeService := dispute.NewDisputeService(pool, dispute_repo.TxRepoFactory(pool.Builder), eventStoreFactory, disputeRepo, silvergateClient, disputeEvents)

	paymentRepo := payment_repo.NewPgPaymentRepo(pool, readDB)
	paymentService := payment.NewPaymentService(pool, payment_repo.TxRepoFactory(pool.Builder), eventStoreFactory, paymentRepo, silvergateClient, cfg.MerchantID)

	// Handlers (clean - no processor dependency)
	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)
	paymentHandler := handlers.NewPaymentHandler(paymentService)

	// Internal handlers (for service-to-service communication)
	updatesHandler := updates.NewUpdatesHandler(orderService, disputeService, paymentService)

	// Health checks registry
	var healthCheckers []health.Checker
	healthCheckers = append(healthCheckers, health.NewPostgresChecker(pool.Pool))
	if readPG != nil {
		healthCheckers = append(healthCheckers, health.NewPostgresChecker(readPG.Pool))
	}
	if cfg.WebhookMode == "kafka" {
		healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
	}
	healthRegistry := health.NewRegistry(healthCheckers...)

	// Router (API endpoints only)
	router := NewRouter(orderHandler, chargebackHandler, disputeHandler, paymentHandler, healthRegistry)
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
		StartWorkers(ctx, cfg, orderService, disputeService, paymentService)
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
