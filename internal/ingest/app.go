package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
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
	l := logger.New(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Setup Gin engine
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(metrics.GinMiddleware(), gin.Recovery())

	// Create processor based on webhook mode
	var processor webhook.Processor
	var closers []io.Closer

	switch cfg.WebhookMode {
	case "kafka":
		l.Info("Webhook mode: kafka - initializing Kafka publishers")
		l.Info("Kafka publishers: brokers=%v, ordersTopic=%s, disputesTopic=%s",
			cfg.KafkaBrokers, cfg.KafkaOrdersTopic, cfg.KafkaDisputesTopic)

		orderPublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaOrdersTopic)
		disputePublisher := kafka.NewPublisher(l, cfg.KafkaBrokers, cfg.KafkaDisputesTopic)
		closers = append(closers, orderPublisher, disputePublisher)

		processor = webhook.NewAsyncProcessor(orderPublisher, disputePublisher)

	case "http":
		l.Info("Webhook mode: http - initializing HTTP client")
		l.Info("API client: baseURL=%s, timeout=%s, retryAttempts=%d",
			cfg.APIBaseURL, cfg.APITimeout, cfg.APIRetryAttempts)

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
		l.Fatal(fmt.Errorf("ingest - Run - unsupported webhook mode: %s (supported: 'kafka', 'http')", cfg.WebhookMode))
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
		l.Info("Ingest service started: port=%d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			l.Error("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	l.Info("Shutting down Ingest service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		l.Error("Server shutdown error: %v", err)
	}

	l.Info("Ingest service stopped")
}
