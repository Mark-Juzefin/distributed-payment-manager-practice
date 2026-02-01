package api

import (
	"TestTaskJustPay/internal/api/handlers"
	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router struct {
	order          *handlers.OrderHandler
	chargeback     *handlers.ChargebackHandler
	dispute        *handlers.DisputeHandler
	healthRegistry *health.Registry
}

func (r *Router) SetUp(engine *gin.Engine) {
	// Health checks (Kubernetes-style)
	engine.GET("/health/live", health.LivenessHandler())
	engine.GET("/health/ready", health.ReadinessHandler(r.healthRegistry, health.DefaultTimeout))

	// Prometheus metrics
	engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	// Manual operations + reads (no webhooks)
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

func NewRouter(
	order *handlers.OrderHandler,
	chargeback *handlers.ChargebackHandler,
	dispute *handlers.DisputeHandler,
	healthRegistry *health.Registry,
) *Router {
	return &Router{
		order:          order,
		chargeback:     chargeback,
		dispute:        dispute,
		healthRegistry: healthRegistry,
	}
}
