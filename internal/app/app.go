package app

import (
	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/controller/http"
	"TestTaskJustPay/internal/controller/http/handlers"
	order_repo "TestTaskJustPay/internal/repo/order"
	"TestTaskJustPay/internal/service/order"
	"TestTaskJustPay/pkg"
	"TestTaskJustPay/pkg/logger"
	"embed"
	"fmt"
	"github.com/gin-gonic/gin"
)

//go:embed migration/*.sql
var MIGRATION_FS embed.FS

type Server struct {
	engine    *gin.Engine
	apiRouter *http.Router
	config    config.Config
}

func Run(cfg *config.Config) {
	l := logger.New(cfg.Log.Level)

	engine := NewGinEngine()

	pool, err := pkg.NewPgPool(cfg.DB.String)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}
	repo := order_repo.NewRepo(pool)
	iOrderService := order.NewOrderService(repo)
	orderHandler := handlers.NewOrderHandler(iOrderService)
	router := http.NewRouter(orderHandler)

	router.SetUp(engine)

	err = applyMigrations(cfg.DB.String, MIGRATION_FS)
	if err != nil {
		l.Fatal(fmt.Errorf("app - Run - postgres.NewPgPool: %w", err))
	}

	if err = engine.Run(); err != nil {
		l.Fatal(fmt.Errorf("app - Run - engine.Run: %w", err))
	}
}
