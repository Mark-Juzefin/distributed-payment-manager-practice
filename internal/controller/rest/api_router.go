package rest

import (
	"TestTaskJustPay/internal/controller/rest/handlers"

	"github.com/gin-gonic/gin"
)

// APIRouter - full router for API service (manual operations + reads + webhooks in sync mode)
type APIRouter struct {
	order           handlers.OrderHandler
	chargeback      handlers.ChargebackHandler
	dispute         handlers.DisputeHandler
	includeWebhooks bool // true in sync mode, false in kafka mode
}

func (r *APIRouter) SetUp(engine *gin.Engine) {
	// Health check
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"service": "api", "status": "ok"})
	})

	// Webhook endpoints only in sync mode
	if r.includeWebhooks {
		engine.POST("/webhooks/payments/orders", r.order.Webhook)
		engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
	}

	// Manual operations + reads
	engine.GET("/orders", r.order.Filter)
	engine.GET("/orders/:order_id", r.order.Get)
	engine.GET("/orders/events", r.order.GetEvents)
	engine.POST("/orders/:order_id/hold", r.order.Hold)
	engine.POST("/orders/:order_id/capture", r.order.Capture)

	engine.GET("/disputes", r.dispute.GetDisputes)
	engine.GET("/disputes/events", r.dispute.GetEvents)
	engine.GET("/disputes/:dispute_id/evidence", r.dispute.GetEvidence)
	engine.POST("/disputes/:dispute_id/evidence", r.dispute.UpsertEvidence)
	engine.POST("/disputes/:dispute_id/submit", r.dispute.Submit)
}

func NewAPIRouter(
	order handlers.OrderHandler,
	chargeback handlers.ChargebackHandler,
	dispute handlers.DisputeHandler,
	includeWebhooks bool,
) *APIRouter {
	return &APIRouter{
		order:           order,
		chargeback:      chargeback,
		dispute:         dispute,
		includeWebhooks: includeWebhooks,
	}
}
