package silvergate

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"TestTaskJustPay/pkg/migrations"
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/services/silvergate/config"
	"TestTaskJustPay/services/silvergate/internal/acquirer"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/product/productrepo"
	"TestTaskJustPay/services/silvergate/internal/purchase"
	"TestTaskJustPay/services/silvergate/internal/transaction"
	"TestTaskJustPay/services/silvergate/internal/transaction/transactioncontroller"
	"TestTaskJustPay/services/silvergate/internal/transaction/transactionrepo"
	"TestTaskJustPay/services/silvergate/internal/webhooksender"

	"github.com/gin-gonic/gin"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrationFS returns the embedded migration files for use in tests.
func MigrationFS() embed.FS { return migrationFS }

type App struct {
	cfg    config.Config
	log    *slog.Logger
	server *http.Server
	pg     *postgres.Postgres
}

func NewApp(cfg config.Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))

	if err := migrations.ApplyMigrations(cfg.PgURL, migrationFS); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	pg, err := postgres.New(cfg.PgURL, postgres.MaxPoolSize(10))
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	txRepo := transactionrepo.NewPgTransactionRepo(pg.Pool)
	acq := acquirer.NewMockAcquirer(cfg.AcquirerAuthApproveRate, cfg.AcquirerSettleSuccessRate, cfg.AcquirerSettleDelay)
	webhookSender := webhooksender.NewSender(cfg.WebhookCallbackURL, log)
	txRepoFactory := func(tx postgres.Executor) transaction.Repo {
		return transactionrepo.NewPgTransactionRepo(tx)
	}
	svc := transaction.NewService(txRepo, acq, webhookSender, log, pg, txRepoFactory)

	authHandler := transactioncontroller.NewAuthHandler(svc)
	captureHandler := transactioncontroller.NewCaptureHandler(svc)
	voidHandler := transactioncontroller.NewVoidHandler(svc)
	refundHandler := transactioncontroller.NewRefundHandler(svc)

	productRepo := productrepo.NewPgProductRepo(pg.Pool)
	productRepoFactory := func(exec postgres.Executor) product.Repo {
		return productrepo.NewPgProductRepo(exec)
	}
	productSvc := product.NewService(productRepo, log, pg, productRepoFactory)

	purchaseSvc := purchase.NewService(productSvc, svc, svc, txRepo, txRepoFactory, pg, log)

	engine := gin.New()
	engine.Use(gin.Recovery())
	setupRouter(engine, authHandler, captureHandler, voidHandler, refundHandler, productSvc, purchaseSvc)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: engine,
	}

	return &App{
		cfg:    cfg,
		log:    log,
		server: server,
		pg:     pg,
	}, nil
}

func (a *App) Run() error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		a.log.Info("silvergate service starting", "port", a.cfg.Port)
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.log.Error("server error", "error", err)
		}
	}()

	<-quit
	a.log.Info("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	a.pg.Close()

	return nil
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
