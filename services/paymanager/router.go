package api

import (
	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/metrics"
	disputectrl "TestTaskJustPay/services/paymanager/dispute/controller"
	orderctrl "TestTaskJustPay/services/paymanager/order/controller"
	paymentctrl "TestTaskJustPay/services/paymanager/payment/controller"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router struct {
	order          *orderctrl.HTTPHandler
	dispute        *disputectrl.HTTPHandler
	payment        *paymentctrl.HTTPHandler
	healthRegistry *health.Registry
}

func NewRouter(
	order *orderctrl.HTTPHandler,
	dispute *disputectrl.HTTPHandler,
	payment *paymentctrl.HTTPHandler,
	healthRegistry *health.Registry,
) *Router {
	return &Router{
		order:          order,
		dispute:        dispute,
		payment:        payment,
		healthRegistry: healthRegistry,
	}
}

func (r *Router) SetUp(engine *gin.Engine) {
	engine.GET("/health/live", health.LivenessHandler())
	engine.GET("/health/ready", health.ReadinessHandler(r.healthRegistry, health.DefaultTimeout))
	engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	// Legacy order endpoints
	engine.GET("/orders", r.order.Filter)
	engine.GET("/orders/:order_id", r.order.Get)
	engine.GET("/orders/events", r.order.GetEvents)
	engine.POST("/orders/:order_id/hold", r.order.Hold)
	engine.POST("/orders/:order_id/capture", r.order.Capture)

	// Dispute endpoints
	engine.GET("/disputes", r.dispute.GetDisputes)
	engine.GET("/disputes/:dispute_id", r.dispute.GetDispute)
	engine.GET("/disputes/events", r.dispute.GetEvents)
	engine.GET("/disputes/:dispute_id/evidence", r.dispute.GetEvidence)
	engine.POST("/disputes/:dispute_id/evidence", r.dispute.UpsertEvidence)
	engine.POST("/disputes/:dispute_id/submit", r.dispute.Submit)

	// Payment endpoints
	engine.POST("/api/v1/payments", r.payment.Create)
	engine.GET("/api/v1/payments/:id", r.payment.Get)
	engine.POST("/api/v1/payments/:id/void", r.payment.Void)
	engine.POST("/api/v1/payments/:id/refund", r.payment.Refund)
}
