package logger

import (
	"bytes"
	"io"

	"github.com/gin-gonic/gin"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
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

		logEvent := l.logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Int("status", c.Writer.Status())

		// Only log bodies for error responses (status >= 400)
		if c.Writer.Status() >= 400 {
			logEvent = logEvent.
				RawJSON("request_body", requestBody).
				RawJSON("response_body", responseBuffer.Bytes())
		}

		logEvent.Msg("HTTP Request")
	}
}
