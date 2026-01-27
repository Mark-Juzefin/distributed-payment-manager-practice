// Package correlation provides utilities for correlation ID propagation.
package correlation

import (
	"context"

	"github.com/google/uuid"
)

// HeaderName is the HTTP header for correlation ID.
const HeaderName = "X-Correlation-ID"

// KafkaHeaderName is the Kafka header for correlation ID.
const KafkaHeaderName = "X-Correlation-ID"

type contextKey struct{}

// FromContext extracts correlation ID from context.
// Returns empty string if not present.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}

// WithID returns a new context with correlation ID.
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// NewID generates a new correlation ID (UUID v4).
func NewID() string {
	return uuid.New().String()
}
