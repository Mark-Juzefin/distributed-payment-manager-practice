package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// skipMetricsPaths contains infrastructure endpoints excluded from business metrics.
var skipMetricsPaths = map[string]bool{
	"/metrics":      true,
	"/health/live":  true,
	"/health/ready": true,
}

// GinMiddleware returns Gin middleware that records HTTP metrics.
func GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if skipMetricsPaths[path] {
			c.Next()
			return
		}

		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		if path == "" {
			path = "unknown"
		}

		HTTPRequestDuration.WithLabelValues(path, c.Request.Method, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(path, c.Request.Method, status).Inc()
	}
}
