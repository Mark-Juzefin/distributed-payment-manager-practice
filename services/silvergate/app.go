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
	"TestTaskJustPay/services/silvergate/acquirer"
	"TestTaskJustPay/services/silvergate/config"
	"TestTaskJustPay/services/silvergate/domain/transaction"
	"TestTaskJustPay/services/silvergate/handlers"
	"TestTaskJustPay/services/silvergate/repo"
	"TestTaskJustPay/services/silvergate/webhook"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrationFS returns the embedded migration files for use in tests.
func MigrationFS() embed.FS { return migrationFS }

type App struct {
	cfg    config.Config
	log    *slog.Logger
	server *http.Server
	pool   *pgxpool.Pool
}

func NewApp(cfg config.Config) (*App, error) {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))

	if err := migrations.ApplyMigrations(cfg.PgURL, migrationFS); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.PgURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	txRepo := repo.NewPgTransactionRepo(pool)
	acq := acquirer.NewMockAcquirer(cfg.AcquirerAuthApproveRate, cfg.AcquirerSettleSuccessRate, cfg.AcquirerSettleDelay)
	webhookSender := webhook.NewSender(cfg.WebhookCallbackURL, log)
	svc := transaction.NewService(txRepo, acq, webhookSender, log)

	authHandler := handlers.NewAuthHandler(svc)
	captureHandler := handlers.NewCaptureHandler(svc)

	engine := gin.New()
	engine.Use(gin.Recovery())
	setupRouter(engine, authHandler, captureHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: engine,
	}

	return &App{
		cfg:    cfg,
		log:    log,
		server: server,
		pool:   pool,
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
	a.pool.Close()

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
