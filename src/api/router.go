package api

import (
	"TestTaskJustPay/src/api/handlers"
	"github.com/gin-gonic/gin"
)

type Router struct {
	order handlers.OrderHandler
}

func (r *Router) SetUp(engine *gin.Engine) {
	engine.GET("/orders", r.order.Filter)
	engine.GET("/orders/:order_id", r.order.Get)
}

func NewRouter(order handlers.OrderHandler) *Router {
	router := &Router{
		order: order,
	}
	return router
}
