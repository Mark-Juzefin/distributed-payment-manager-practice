package logger

import (
	"bytes"
	"encoding/json"
	"io"

	"TestTaskJustPay/pkg/correlation"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const maxBody = 8 * 1024 // 8KB

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

func (l *Logger) GinBodyLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
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

		logEvent := l.logger.Info()

		if corrID := correlation.FromContext(c.Request.Context()); corrID != "" {
			logEvent = logEvent.Str("correlation_id", corrID)
		}

		logEvent = logEvent.
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Int("status", c.Writer.Status())

		//// Only log bodies for error responses (status >= 400)
		//if c.Writer.Status() >= 400 {
		logEvent = addMaybeJSON(logEvent, "request_body", limit(requestBody))
		logEvent = addMaybeJSON(logEvent, "response_body", limit(responseBuffer.Bytes()))
		//}

		logEvent.Msg("HTTP Request")
	}
}

func addMaybeJSON(e *zerolog.Event, key string, b []byte) *zerolog.Event {
	bb := bytes.TrimSpace(b)

	// порожнє тіло -> null
	if len(bb) == 0 {
		return e.RawJSON(key, []byte("null"))
	}

	// валідний JSON -> вставляємо як JSON
	if json.Valid(bb) {
		return e.RawJSON(key, bb)
	}

	// не JSON -> як строка (інакше зламає формат)
	return e.Str(key, string(bb))
}
