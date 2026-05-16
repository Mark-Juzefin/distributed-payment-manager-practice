package api

import (
	"TestTaskJustPay/services/paymanager/updates"

	"github.com/gin-gonic/gin"
)

// InternalRouter sets up routes for internal service-to-service communication.
type InternalRouter struct {
	updates *updates.Handler
}

func NewInternalRouter(updates *updates.Handler) *InternalRouter {
	return &InternalRouter{
		updates: updates,
	}
}

func (r *InternalRouter) SetUp(engine *gin.Engine) {
	internalGroup := engine.Group("/internal")
	{
		internalGroup.POST("/updates/orders", r.updates.HandleOrderUpdate)
		internalGroup.POST("/updates/disputes", r.updates.HandleDisputeUpdate)
		internalGroup.POST("/updates/payments", r.updates.HandlePaymentWebhook)
	}
}
