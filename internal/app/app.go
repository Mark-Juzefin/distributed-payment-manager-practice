package app

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/rest"
	"TestTaskJustPay/internal/controller/rest/handlers"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/external/kafka"
	"TestTaskJustPay/internal/external/silvergate"
	dispute_repo "TestTaskJustPay/internal/repo/dispute"
	"TestTaskJustPay/internal/repo/dispute_eventsink"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/internal/repo/order_eventsink"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
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
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
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

	// Kafka publishers
	orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
	disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
	defer orderPublisher.Close()
	defer disputePublisher.Close()

	// Services with publishers
	orderService := order.NewOrderService(orderRepo, silvergateClient, orderEventSink, orderPublisher)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, disputeEventSink, disputePublisher)

	// Handlers
	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	router := rest.NewRouter(orderHandler, chargebackHandler, disputeHandler)

	router.SetUp(engine)

	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - ApplyMigrations: %w", err))
	}

	// Start Kafka consumers
	StartWorkers(ctx, l, cfg, orderService, disputeService)

	// Start HTTP server in a goroutine
	go func() {
		l.Info("Starting HTTP server: port=%d", cfg.Port)
		if err := engine.Run(fmt.Sprintf(":%d", cfg.Port)); err != nil {
			l.Error("HTTP server error: error=%v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	l.Info("Shutting down gracefully...")
}
