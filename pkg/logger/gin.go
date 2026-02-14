package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"

	"TestTaskJustPay/pkg/correlation"

	"github.com/gin-gonic/gin"
)

const maxBody = 8 * 1024 // 8KB

// skipLogPaths contains paths where request logging should be skipped entirely.
// These are high-frequency infrastructure endpoints (health checks, metrics)
// that create noise in logs.
var skipLogPaths = map[string]bool{
	"/metrics":      true,
	"/health/live":  true,
	"/health/ready": true,
}

func shouldSkipLog(path string) bool {
	return skipLogPaths[path]
}

func limit(b []byte) []byte {
	if len(b) > maxBody {
		return b[:maxBody]
	}
	return b
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// CorrelationMiddleware extracts X-Correlation-ID from request header or generates a new one.
// It stores the ID in the request context and adds it to the response header.
func CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		corrID := c.GetHeader(correlation.HeaderName)
		if corrID == "" {
			corrID = correlation.NewID()
		}

		// Store in request context (accessible via c.Request.Context())
		ctx := correlation.WithID(c.Request.Context(), corrID)
		c.Request = c.Request.WithContext(ctx)

		// Add to response header
		c.Header(correlation.HeaderName, corrID)

		c.Next()
	}
}

// GinBodyLogger returns a Gin middleware that logs HTTP request/response details.
func GinBodyLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if shouldSkipLog(path) {
			c.Next()
			return
		}

		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		responseBuffer := &bytes.Buffer{}
		writer := &responseBodyWriter{
			body:           responseBuffer,
			ResponseWriter: c.Writer,
		}
		c.Writer = writer

		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"query", c.Request.URL.RawQuery,
			"status", c.Writer.Status(),
		}
		attrs = append(attrs, bodyAttr("request_body", limit(requestBody))...)
		attrs = append(attrs, bodyAttr("response_body", limit(responseBuffer.Bytes()))...)

		slog.InfoContext(c.Request.Context(), "HTTP Request", attrs...)
	}
}

// bodyAttr returns the appropriate log attributes for a body payload.
func bodyAttr(key string, b []byte) []any {
	bb := bytes.TrimSpace(b)

	if len(bb) == 0 {
		return []any{key, nil}
	}

	if json.Valid(bb) {
		// For valid JSON, log as raw JSON by unmarshaling first
		var v any
		if err := json.Unmarshal(bb, &v); err == nil {
			return []any{key, v}
		}
	}

	// Not JSON or unmarshal failed - log as string
	return []any{key, string(bb)}
}
