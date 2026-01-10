package api

import (
	"TestTaskJustPay/internal/api/handlers/updates"

	"github.com/gin-gonic/gin"
)

// InternalRouter sets up routes for internal service-to-service communication.
// These endpoints are used by Ingest service to forward webhooks to API service.
type InternalRouter struct {
	updates *updates.UpdatesHandler
}

func NewInternalRouter(updates *updates.UpdatesHandler) *InternalRouter {
	return &InternalRouter{
		updates: updates,
	}
}

// SetUp registers internal routes on the Gin engine.
func (r *InternalRouter) SetUp(engine *gin.Engine) {
	internalGroup := engine.Group("/internal")
	{
		internalGroup.POST("/updates/orders", r.updates.HandleOrderUpdate)
		internalGroup.POST("/updates/disputes", r.updates.HandleDisputeUpdate)
	}
}
