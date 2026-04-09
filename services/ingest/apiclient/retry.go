package apiclient

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry with exponential backoff.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryConfig returns sensible defaults for retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    5 * time.Second,
	}
}

// DoWithRetry executes the given function with exponential backoff retry logic.
// It only retries on ErrServiceUnavailable errors.
func DoWithRetry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on service unavailable errors
		if !errors.Is(err, ErrServiceUnavailable) {
			return err
		}

		// Don't wait after the last attempt
		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := calculateBackoff(attempt, cfg.BaseDelay, cfg.MaxDelay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateBackoff computes exponential backoff with jitter.
func calculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential: baseDelay * 2^attempt
	delay := float64(baseDelay) * math.Pow(2, float64(attempt))

	// Add jitter (±25%)
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter

	// Cap at maxDelay
	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	return time.Duration(delay)
}
