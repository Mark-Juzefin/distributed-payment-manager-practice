package messaging

import (
	"context"
	"log/slog"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// Runner manages multiple workers and runs them concurrently.
type Runner struct {
	workers []Worker
	handler MessageHandler
}

// NewRunner creates a new runner with the given workers and handler.
func NewRunner(workers []Worker, handler MessageHandler) *Runner {
	return &Runner{
		workers: workers,
		handler: handler,
	}
}

// Start runs all workers concurrently and waits for them to finish.
// Returns when context is cancelled or any worker returns an error.
func (r *Runner) Start(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)

	for i, w := range r.workers {
		i, w := i, w
		g.Go(func() error {
			defer func() {
				if rec := recover(); rec != nil {
					slog.Error("Worker panic recovered",
						"worker_idx", i,
						"panic", rec,
						"stack", string(debug.Stack()))
				}
				if err := w.Close(); err != nil {
					slog.Error("Failed to close worker",
						"worker_idx", i,
						slog.Any("error", err))
				}
			}()
			return w.Start(ctx, r.handler)
		})
	}

	return g.Wait()
}

// Close closes all workers.
func (r *Runner) Close() error {
	for _, w := range r.workers {
		if err := w.Close(); err != nil {
			slog.Error("Failed to close worker", slog.Any("error", err))
		}
	}
	return nil
}
