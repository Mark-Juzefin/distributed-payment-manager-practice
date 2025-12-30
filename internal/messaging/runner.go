package messaging

import (
	"context"
	"runtime/debug"

	"TestTaskJustPay/pkg/logger"

	"golang.org/x/sync/errgroup"
)

// Runner manages multiple workers and runs them concurrently.
type Runner struct {
	logger  *logger.Logger
	workers []Worker
	handler MessageHandler
}

// NewRunner creates a new runner with the given workers and handler.
func NewRunner(l *logger.Logger, workers []Worker, handler MessageHandler) *Runner {
	return &Runner{
		logger:  l,
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
					r.logger.Error("Worker panic recovered: worker_idx=%d panic=%v stack=%s",
						i, rec, string(debug.Stack()))
				}
				if err := w.Close(); err != nil {
					r.logger.Error("Failed to close worker: worker_idx=%d error=%v", i, err)
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
			r.logger.Error("Failed to close worker: error=%v", err)
		}
	}
	return nil
}
