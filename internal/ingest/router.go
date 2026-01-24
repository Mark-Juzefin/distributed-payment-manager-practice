package ingest

import (
	"TestTaskJustPay/internal/ingest/handlers"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

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
