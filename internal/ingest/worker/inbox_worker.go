package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"TestTaskJustPay/internal/ingest/apiclient"
	"TestTaskJustPay/internal/ingest/repo/inbox"
	"TestTaskJustPay/internal/shared/dto"
)

// Config holds configuration for the inbox worker.
type Config struct {
	PollInterval time.Duration
	BatchSize    int
	MaxRetries   int
}

// InboxWorker polls the inbox table for pending messages and forwards them to the API.
type InboxWorker struct {
	repo   inbox.InboxRepo
	client apiclient.Client
	cfg    Config
}

// NewInboxWorker creates a new inbox worker.
func NewInboxWorker(repo inbox.InboxRepo, client apiclient.Client, cfg Config) *InboxWorker {
	return &InboxWorker{
		repo:   repo,
		client: client,
		cfg:    cfg,
	}
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (w *InboxWorker) Start(ctx context.Context) error {
	slog.Info("Inbox worker started",
		"poll_interval", w.cfg.PollInterval,
		"batch_size", w.cfg.BatchSize,
		"max_retries", w.cfg.MaxRetries)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Inbox worker stopped")
			return ctx.Err()
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *InboxWorker) poll(ctx context.Context) {
	messages, err := w.repo.FetchPending(ctx, w.cfg.BatchSize)
	if err != nil {
		slog.Error("Failed to fetch pending inbox messages", slog.Any("error", err))
		return
	}

	if len(messages) == 0 {
		return
	}

	slog.Debug("Processing inbox batch", "count", len(messages))

	for _, msg := range messages {
		if err := w.processMessage(ctx, msg); err != nil {
			slog.Warn("Inbox message processing failed",
				"id", msg.ID,
				"webhook_type", msg.WebhookType,
				"retry_count", msg.RetryCount,
				slog.Any("error", err))

			maxRetries := w.cfg.MaxRetries
			if isPermanentError(err) {
				maxRetries = 0 // force immediate failure
			}

			if markErr := w.repo.MarkFailed(ctx, msg.ID, err.Error(), maxRetries); markErr != nil {
				slog.Error("Failed to mark inbox message as failed",
					"id", msg.ID, slog.Any("error", markErr))
			}
		} else {
			if markErr := w.repo.MarkProcessed(ctx, msg.ID); markErr != nil {
				slog.Error("Failed to mark inbox message as processed",
					"id", msg.ID, slog.Any("error", markErr))
			}
		}
	}
}

func (w *InboxWorker) processMessage(ctx context.Context, msg inbox.InboxMessage) error {
	switch msg.WebhookType {
	case "order_update":
		var req dto.OrderUpdateRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return fmt.Errorf("unmarshal order update: %w", err)
		}
		err := w.client.SendOrderUpdate(ctx, req)
		if errors.Is(err, apiclient.ErrConflict) {
			return nil // already processed — idempotent success
		}
		return err

	case "dispute_update":
		var req dto.DisputeUpdateRequest
		if err := json.Unmarshal(msg.Payload, &req); err != nil {
			return fmt.Errorf("unmarshal dispute update: %w", err)
		}
		err := w.client.SendDisputeUpdate(ctx, req)
		if errors.Is(err, apiclient.ErrConflict) {
			return nil // already processed — idempotent success
		}
		return err

	default:
		return fmt.Errorf("unknown webhook type: %s", msg.WebhookType)
	}
}

// isPermanentError returns true for errors that should not be retried.
func isPermanentError(err error) bool {
	return errors.Is(err, apiclient.ErrBadRequest) ||
		errors.Is(err, apiclient.ErrNotFound) ||
		errors.Is(err, apiclient.ErrInvalidStatus)
}
