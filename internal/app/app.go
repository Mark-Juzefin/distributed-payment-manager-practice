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

//go:embed migration/*.sql
var MIGRATION_FS embed.FS

func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	engine := NewGinEngine()

	pool, err := postgres.New(cfg.PG.String, postgres.MaxPoolSize(cfg.PG.PoolMax))
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}
	repo := order_repo.NewPgOrderRepo(pool)
	iOrderService := order.NewOrderService(repo)
	orderHandler := handlers.NewOrderHandler(iOrderService)
	router := http.NewRouter(orderHandler)

	router.SetUp(engine)

	err = applyMigrations(cfg.PG.String, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}

	if err = engine.Run(); err != nil {
		l.Fatal(fmt.Errorf("app - Run - engine.Run: %w", err))
	}
}
