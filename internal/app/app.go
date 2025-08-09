package app

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/http"
	"TestTaskJustPay/internal/controller/http/handlers"
	"TestTaskJustPay/internal/domain/order"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/postgres"
	"embed"
	"fmt"
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
	repo := order_repo.NewPgOrderRepo(pool)
	iOrderService := order.NewOrderService(repo)
	orderHandler := handlers.NewOrderHandler(iOrderService)
	router := http.NewRouter(orderHandler)

	router.SetUp(engine)

	err = ApplyMigrations(cfg.PgURL, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}

	if err = engine.Run(); err != nil {
		l.Fatal(fmt.Errorf("app - Run - engine.Run: %w", err))
	}
}
