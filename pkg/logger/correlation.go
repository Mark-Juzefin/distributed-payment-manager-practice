package logger

import (
	"context"
	"log/slog"

	"TestTaskJustPay/pkg/correlation"
)

// CorrelationHandler wraps an slog.Handler to automatically inject
// correlation_id from the context into every log record.
type CorrelationHandler struct {
	inner slog.Handler
}

// NewCorrelationHandler creates a handler that adds correlation_id from context.
func NewCorrelationHandler(inner slog.Handler) *CorrelationHandler {
	return &CorrelationHandler{inner: inner}
}

// Handle adds correlation_id from context before delegating to inner handler.
func (h *CorrelationHandler) Handle(ctx context.Context, r slog.Record) error {
	if corrID := correlation.FromContext(ctx); corrID != "" {
		r.AddAttrs(slog.String("correlation_id", corrID))
	}
	return h.inner.Handle(ctx, r)
}

// Enabled delegates to the inner handler.
func (h *CorrelationHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

// WithAttrs returns a new handler with the given attributes.
func (h *CorrelationHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CorrelationHandler{inner: h.inner.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name.
func (h *CorrelationHandler) WithGroup(name string) slog.Handler {
	return &CorrelationHandler{inner: h.inner.WithGroup(name)}
}
