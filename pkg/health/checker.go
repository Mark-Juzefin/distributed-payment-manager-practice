package health

import (
	"context"
	"time"
)

// DefaultTimeout is the default timeout for health checks.
const DefaultTimeout = 5 * time.Second

// Status represents the health status of a component.
type Status string

const (
	StatusUp   Status = "up"
	StatusDown Status = "down"
)

// Result is the outcome of a single health check.
type Result struct {
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// Checker is the interface for health check implementations.
type Checker interface {
	// Name returns the name of the component being checked.
	Name() string
	// Check performs the health check and returns the result.
	Check(ctx context.Context) Result
}
