// Package merchantauth provides a placeholder merchant-identity middleware.
// It reads an X-Merchant-ID header and surfaces the value via context, so
// downstream handlers stay decoupled from the eventual JWT/API-key auth.
package merchantauth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

const HeaderName = "X-Merchant-ID"

type contextKey struct{}

// FromContext returns the merchant id stored in ctx by Middleware.
// The boolean is false when no merchant id was set.
func FromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(contextKey{}).(string)
	if !ok || v == "" {
		return "", false
	}
	return v, true
}

// WithMerchantID returns a new context carrying merchantID.
// Exposed so tests can stub merchant identity without invoking the middleware.
func WithMerchantID(ctx context.Context, merchantID string) context.Context {
	return context.WithValue(ctx, contextKey{}, merchantID)
}

// Middleware aborts with 401 when X-Merchant-ID is absent; otherwise it injects
// the value into the request context.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderName)
		if id == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing X-Merchant-ID header",
			})
			return
		}
		c.Request = c.Request.WithContext(WithMerchantID(c.Request.Context(), id))
		c.Next()
	}
}
