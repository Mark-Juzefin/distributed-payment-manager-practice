package ingest

import (
	"TestTaskJustPay/internal/ingest/handlers"
	"TestTaskJustPay/pkg/health"
	"TestTaskJustPay/pkg/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Router struct {
	order          *handlers.OrderHandler
	chargeback     *handlers.ChargebackHandler
	healthRegistry *health.Registry
}

func (r *Router) SetUp(engine *gin.Engine) {
	// Health checks (Kubernetes-style)
	engine.GET("/health/live", health.LivenessHandler())
	engine.GET("/health/ready", health.ReadinessHandler(r.healthRegistry, health.DefaultTimeout))

	engine.GET("/metrics", gin.WrapH(promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{})))

	// Webhook endpoints only
	engine.POST("/webhooks/payments/orders", r.order.Webhook)
	engine.POST("/webhooks/payments/chargebacks", r.chargeback.Webhook)
}

func NewRouter(order *handlers.OrderHandler, chargeback *handlers.ChargebackHandler, healthRegistry *health.Registry) *Router {
	return &Router{
		order:          order,
		chargeback:     chargeback,
		healthRegistry: healthRegistry,
	}
}
