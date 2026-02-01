package ingest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"TestTaskJustPay/config"
	"TestTaskJustPay/internal/api/external/kafka"
	"TestTaskJustPay/internal/ingest/apiclient"
	"TestTaskJustPay/internal/ingest/handlers"
	"TestTaskJustPay/internal/ingest/webhook"
	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/logger"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
)

// Run bootstraps and runs the Ingest service
func Run(cfg config.IngestConfig) {
	logger.Setup(logger.Options{
		Level:   cfg.LogLevel,
		Console: strings.ToLower(os.Getenv("LOG_FORMAT")) == "console",
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Setup Gin engine
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(
		logger.CorrelationMiddleware(), // Extract/generate correlation ID first
		metrics.GinMiddleware(),
		logger.GinBodyLogger(),
		gin.Recovery(),
	)

	// Create processor based on webhook mode
	var processor webhook.Processor
	var closers []io.Closer

	switch cfg.WebhookMode {
	case "kafka":
		slog.Info("Webhook mode: kafka - initializing Kafka publishers")
		slog.Info("Kafka publishers configured",
			"brokers", cfg.KafkaBrokers,
			"orders_topic", cfg.KafkaOrdersTopic,
			"disputes_topic", cfg.KafkaDisputesTopic)

		orderPublisher := kafka.NewPublisher(cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
		disputePublisher := kafka.NewPublisher(cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
		closers = append(closers, orderPublisher, disputePublisher)

		processor = webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

	case "http":
		slog.Info("Webhook mode: http - initializing HTTP client")
		slog.Info("API client configured",
			"base_url", cfg.APIBaseURL,
			"timeout", cfg.APITimeout,
			"retry_attempts", cfg.APIRetryAttempts)

		client := apiclient.NewHTTPClient(apiclient.HTTPClientConfig{
			BaseURL:        cfg.APIBaseURL,
			Timeout:        cfg.APITimeout,
			RetryAttempts:  cfg.APIRetryAttempts,
			RetryBaseDelay: cfg.APIRetryBaseDelay,
			RetryMaxDelay:  cfg.APIRetryMaxDelay,
		})
		closers = append(closers, client)

		processor = webhook.NewHTTPSyncProcessor(client)

	default:
		slog.Error("Unsupported webhook mode",
			"mode", cfg.WebhookMode,
			"supported", []string{"kafka", "http"})
		os.Exit(1)
	}

	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()

	// Handlers (processor only, clean)
	orderHandler := handlers.NewOrderHandler(processor)
	chargebackHandler := handlers.NewChargebackHandler(processor)

	// Health checks registry
	var healthCheckers []health.Checker
	if cfg.WebhookMode == "kafka" {
		healthCheckers = append(healthCheckers, health.NewKafkaChecker(cfg.KafkaBrokers))
	}
	// HTTP mode: no external dependencies to check
	healthRegistry := health.NewRegistry(healthCheckers...)

	// Webhook-only routes
	router := NewRouter(orderHandler, chargebackHandler, healthRegistry)
	router.SetUp(engine)

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: engine,
	}

	go func() {
		slog.Info("Ingest service started", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", slog.Any("error", err))
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down Ingest service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", slog.Any("error", err))
	}

	slog.Info("Ingest service stopped")
}
