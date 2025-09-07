package app

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/rest"
	"TestTaskJustPay/internal/controller/rest/handlers"
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/external/silvergate"
	dispute_repo "TestTaskJustPay/internal/repo/dispute"
	"TestTaskJustPay/internal/repo/dispute_eventsink"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/internal/repo/order_eventsink"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"embed"
	"fmt"
	"net/http"
)

//go:embed migrations/*.sql
var MIGRATION_FS embed.FS

func Run(cfg config.Config) {
	l := logger.New(cfg.LogLevel)

	engine := NewGinEngine(l)

	pool, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(cfg.PgPoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}
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
	orderService := order.NewOrderService(orderRepo, silvergateClient, orderEventSink)
	disputeService := dispute.NewDisputeService(disputeRepo, silvergateClient, disputeEventSink)

	orderHandler := handlers.NewOrderHandler(orderService)
	chargebackHandler := handlers.NewChargebackHandler(disputeService)
	disputeHandler := handlers.NewDisputeHandler(disputeService)

	router := rest.NewRouter(orderHandler, chargebackHandler, disputeHandler)

	router.SetUp(engine)

	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}

	if err = engine.Run(); err != nil {
		l.Fatal(fmt.Errorf("app - Run - engine.Run: %w", err))
	}
}
