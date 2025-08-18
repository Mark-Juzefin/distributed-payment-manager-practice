package rest

import (
	"TestTaskJustPay/internal/controller/rest/handlers"

	"github.com/gin-gonic/gin"
)

type Router struct {
	order      handlers.OrderHandler
	chargeback handlers.ChargebackHandler
	dispute    handlers.DisputeHandler
}

func (r *Router) SetUp(engine *gin.Engine) {
	engine.POST("/webhooks/payments/orders", r.order.Webhook)
	engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)

	engine.GET("/orders", r.order.Filter)
	engine.GET("/orders/:order_id", r.order.Get)
	engine.GET("/orders/:order_id/events", r.order.GetEvents)

	engine.GET("/disputes", r.dispute.GetDisputes)
	engine.GET("/disputes/:dispute_id/events", r.dispute.GetEvents)
	engine.GET("/disputes/:dispute_id/evidence", r.dispute.GetEvidence)
	engine.POST("/disputes/:dispute_id/evidence", r.dispute.UpsertEvidence)
}

func NewRouter(order handlers.OrderHandler, chargeback handlers.ChargebackHandler, dispute handlers.DisputeHandler) *Router {
	router := &Router{
		order:      order,
		chargeback: chargeback,
		dispute:    dispute,
	}
	return router
}
