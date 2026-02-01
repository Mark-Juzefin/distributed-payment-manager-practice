package health

import (
	"context"
	"sync"
)

// Registry holds multiple health checkers.
type Registry struct {
	checkers []Checker
}

// NewRegistry creates a new health check registry.
func NewRegistry(checkers ...Checker) *Registry {
	return &Registry{checkers: checkers}
}

// CheckResult is the result of a single named check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
}

// ReadinessResponse is the aggregated readiness check response.
type ReadinessResponse struct {
	Status Status        `json:"status"`
	Checks []CheckResult `json:"checks,omitempty"`
}

// CheckAll runs all registered checkers in parallel.
func (r *Registry) CheckAll(ctx context.Context) ReadinessResponse {
	if len(r.checkers) == 0 {
		return ReadinessResponse{Status: StatusUp}
	}

	results := make([]CheckResult, len(r.checkers))
	var wg sync.WaitGroup

	for i, checker := range r.checkers {
		wg.Add(1)
		go func(idx int, c Checker) {
			defer wg.Done()
			res := c.Check(ctx)
			results[idx] = CheckResult{
				Name:    c.Name(),
				Status:  res.Status,
				Message: res.Message,
			}
		}(i, checker)
	}

	wg.Wait()

	overall := StatusUp
	for _, res := range results {
		if res.Status == StatusDown {
			overall = StatusDown
			break
		}
	}

	return ReadinessResponse{Status: overall, Checks: results}
}
