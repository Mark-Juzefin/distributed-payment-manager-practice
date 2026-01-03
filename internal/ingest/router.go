package ingest

import (
	"TestTaskJustPay/internal/ingest/handlers"

	"github.com/gin-gonic/gin"
)

type Router struct {
	order      *handlers.OrderHandler
	chargeback *handlers.ChargebackHandler
}

func (r *Router) SetUp(engine *gin.Engine) {
	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"service": "ingest", "status": "ok"})
	})

	// Webhook endpoints only
	engine.POST("/webhooks/payments/orders", r.order.Webhook)
	engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
}

func NewRouter(order *handlers.OrderHandler, chargeback *handlers.ChargebackHandler) *Router {
	return &Router{
		order:      order,
		chargeback: chargeback,
	}
}
