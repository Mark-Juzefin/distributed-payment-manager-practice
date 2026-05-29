package paymanager

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
	"TestTaskJustPay/services/paymanager/internal/dispute"
	"TestTaskJustPay/services/paymanager/internal/dispute/disputecontroller"
	"TestTaskJustPay/services/paymanager/internal/dispute/disputerepo"
	"TestTaskJustPay/services/paymanager/internal/eventstore"
	"TestTaskJustPay/services/paymanager/internal/order"
	"TestTaskJustPay/services/paymanager/internal/order/ordercontroller"
	"TestTaskJustPay/services/paymanager/internal/order/orderrepo"
	"TestTaskJustPay/services/paymanager/internal/payment"
	"TestTaskJustPay/services/paymanager/internal/payment/paymentcontroller"
	"TestTaskJustPay/services/paymanager/internal/payment/paymentrepo"
	"TestTaskJustPay/services/paymanager/internal/silvergateclient"
)

//go:embed migrations/*.sql
var MIGRATION_FS embed.FS

func Run(cfg config.Config) {
	logger.Setup(logger.Options{
		Level:   cfg.LogLevel,
		Console: strings.ToLower(os.Getenv("LOG_FORMAT")) == "console",
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	engine := NewGinEngine()

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		slog.Error("Failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

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

	// Repositories
	orderRepo := orderrepo.New(pool, readDB)
	orderEvents := orderrepo.NewEventSink(pool.Pool, readDB, pool.Builder)
	disputeRepo := disputerepo.New(pool, readDB)
	disputeEvents := disputerepo.NewEventSink(pool.Pool, readDB, pool.Builder)
	paymentRepo := paymentrepo.New(pool, readDB)

	silvergateClient := silvergateclient.New(
		cfg.SilvergateBaseURL,
		cfg.SilvergateSubmitRepresentmentPath,
		cfg.SilvergateCapturePath,
		cfg.SilvergateAuthPath,
		cfg.SilvergateVoidPath,
		&http.Client{Timeout: cfg.HTTPSilvergateClientTimeout},
	)

	// Event store factory (shared across services)
	eventStoreFactory := eventstore.TxStoreFactory(pool.Builder)

	// Services
	orderService := order.NewOrderService(
		pool,
		orderrepo.TxRepoFactory(pool.Builder),
		eventStoreFactory,
		orderRepo,
		silvergateClient,
		orderEvents,
	)
	disputeService := dispute.NewDisputeService(
		pool,
		disputerepo.TxRepoFactory(pool.Builder),
		eventStoreFactory,
		disputeRepo,
		silvergateClient,
		disputeEvents,
	)
	paymentService := payment.NewPaymentService(
		pool,
		paymentrepo.TxRepoFactory(pool.Builder),
		eventStoreFactory,
		paymentRepo,
		silvergateClient,
		cfg.MerchantID,
	)

	// Handlers
	orderH := ordercontroller.NewHTTPHandler(orderService)
	disputeH := disputecontroller.NewHTTPHandler(disputeService)
	paymentH := paymentcontroller.NewHTTPHandler(paymentService)

	// Health checks
	var healthCheckers []health.Checker
	healthCheckers = append(healthCheckers, health.NewPostgresChecker(pool.Pool))
	if readPG != nil {
		healthCheckers = append(healthCheckers, health.NewPostgresChecker(readPG.Pool))
	}
	if cfg.WebhookMode == "kafka" {
		healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
	}
	healthRegistry := health.NewRegistry(healthCheckers...)

	// Routers
	router := NewRouter(orderH, disputeH, paymentH, healthRegistry)
	router.SetUp(engine)

	internalRouter := NewInternalRouter(orderH, disputeH, paymentH)
	internalRouter.SetUp(engine)

	// Migrations
	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		slog.Error("Failed to apply migrations", slog.Any("error", err))
		os.Exit(1)
	}

	// Kafka consumers
	if cfg.WebhookMode == "kafka" {
		slog.Info("Webhook mode: kafka - starting Kafka consumers")
		StartWorkers(ctx, cfg, orderService, disputeService, paymentService)
	}

	go func() {
		slog.Info("Starting API HTTP server", "port", cfg.Port)
		if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			slog.Error("HTTP server error", slog.Any("error", err))
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down API service gracefully...")
}

// Compile-time checks: silvergate client must satisfy all domain Provider interfaces.
var (
	_ order.Provider   = (*silvergateclient.Client)(nil)
	_ dispute.Provider = (*silvergateclient.Client)(nil)
	_ payment.Provider = (*silvergateclient.Client)(nil)
)
