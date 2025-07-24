package http

import (
	"TestTaskJustPay/internal/controller/http/handlers"
	"github.com/gin-gonic/gin"
)

type Router struct {
	order handlers.OrderHandler
}

func (r *Router) SetUp(engine *gin.Engine) {
	engine.POST("/webhooks/payments/orders", r.order.Webhook)

	engine.GET("/orders", r.order.Filter)
	engine.GET("/orders/:order_id", r.order.Get)
	engine.GET("/orders/:order_id/events", r.order.GetEvents)
}

func NewRouter(order handlers.OrderHandler) *Router {
	router := &Router{
		order: order,
	}
	return router
}
