package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// LivenessHandler returns a handler for liveness probes.
// Always returns 200 OK if the process is running.
func LivenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": StatusUp})
	}
}

// ReadinessHandler returns a handler for readiness probes.
// Returns 200 OK if all checks pass, 503 Service Unavailable otherwise.
func ReadinessHandler(registry *Registry, timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		response := registry.CheckAll(ctx)

		status := http.StatusOK
		if response.Status == StatusDown {
			status = http.StatusServiceUnavailable
		}

		c.JSON(status, response)
	}
}
