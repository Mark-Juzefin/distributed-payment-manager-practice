package paymanager

import (
	"TestTaskJustPay/services/paymanager/internal/dispute/disputecontroller"
	"TestTaskJustPay/services/paymanager/internal/order/ordercontroller"
	"TestTaskJustPay/services/paymanager/internal/payment/paymentcontroller"

	"github.com/gin-gonic/gin"
)

// InternalRouter sets up routes for internal service-to-service communication
// (webhook updates forwarded from the Ingest service).
type InternalRouter struct {
	order   *ordercontroller.HTTPHandler
	dispute *disputecontroller.HTTPHandler
	payment *paymentcontroller.HTTPHandler
}

func NewInternalRouter(
	order *ordercontroller.HTTPHandler,
	dispute *disputecontroller.HTTPHandler,
	payment *paymentcontroller.HTTPHandler,
) *InternalRouter {
	return &InternalRouter{
		order:   order,
		dispute: dispute,
		payment: payment,
	}
}

func (r *InternalRouter) SetUp(engine *gin.Engine) {
	internalGroup := engine.Group("/internal")
	{
		internalGroup.POST("/updates/orders", r.order.HandleUpdate)
		internalGroup.POST("/updates/disputes", r.dispute.HandleUpdate)
		internalGroup.POST("/updates/payments", r.payment.HandleWebhook)
	}
}
