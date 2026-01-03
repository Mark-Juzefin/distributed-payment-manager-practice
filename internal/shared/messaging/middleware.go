package messaging

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

const dlqPublishTimeout = 5 * time.Second

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
	}
}

// ErrMaxRetriesExceeded is returned when all retry attempts fail.
var ErrMaxRetriesExceeded = errors.New("max retries exceeded")

// WithRetry wraps a handler with exponential backoff + jitter retry logic.
func WithRetry(handler MessageHandler, cfg RetryConfig) MessageHandler {
	return func(ctx context.Context, key, value []byte) error {
		backoff := cfg.InitialBackoff

		var lastErr error
		for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
			lastErr = handler(ctx, key, value)
			if lastErr == nil {
				return nil
			}

			// Don't sleep after last attempt
			if attempt < cfg.MaxAttempts-1 {
				// Add jitter: backoff + random(0-100ms)
				jitter := time.Duration(rand.Intn(100)) * time.Millisecond
				sleepTime := backoff + jitter
				if sleepTime > cfg.MaxBackoff {
					sleepTime = cfg.MaxBackoff
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(sleepTime):
				}

				backoff *= 2 // exponential
			}
		}

		return errors.Join(ErrMaxRetriesExceeded, lastErr)
	}
}

// DLQPublisher can publish failed messages to a dead letter queue.
type DLQPublisher interface {
	PublishToDLQ(ctx context.Context, key, value []byte, err error) error
}

// WithDLQ wraps a handler to send failed messages to DLQ after exhausting retries.
func WithDLQ(handler MessageHandler, dlq DLQPublisher) MessageHandler {
	return func(ctx context.Context, key, value []byte) error {
		err := handler(ctx, key, value)
		if err != nil {
			// Use separate context to ensure DLQ publish completes even during shutdown.
			// Main ctx may be cancelled, but we still want to persist the failed message.
			dlqCtx, cancel := context.WithTimeout(context.Background(), dlqPublishTimeout)
			defer cancel()
			// Ignore DLQ publish errors (logged in implementation)
			_ = dlq.PublishToDLQ(dlqCtx, key, value, err)
			// Return nil so consumer commits offset - message is now in DLQ
			return nil
		}
		return nil
	}
}
