package rest

import (
	"TestTaskJustPay/internal/controller/rest/handlers"

	"github.com/gin-gonic/gin"
)

// WebhookRouter - lightweight router for Ingest service (webhooks only)
type WebhookRouter struct {
	order      handlers.OrderHandler
	chargeback handlers.ChargebackHandler
}

func (r *WebhookRouter) SetUp(engine *gin.Engine) {
	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"service": "ingest", "status": "ok"})
	})

	// Webhook endpoints only
	engine.POST("/webhooks/payments/orders", r.order.Webhook)
	engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
}

func NewWebhookRouter(order handlers.OrderHandler, chargeback handlers.ChargebackHandler) *WebhookRouter {
	return &WebhookRouter{
		order:      order,
		chargeback: chargeback,
	}
}
