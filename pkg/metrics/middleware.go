package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GinMiddleware returns Gin middleware that records HTTP metrics.
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		handler := c.FullPath()
		if handler == "" {
			handler = "unknown"
		}

		HTTPRequestDuration.WithLabelValues(handler, c.Request.Method, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(handler, c.Request.Method, status).Inc()
	}
}
