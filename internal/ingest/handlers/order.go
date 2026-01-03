package handlers

import (
	"TestTaskJustPay/internal/shared/domain/order"
	"TestTaskJustPay/internal/shared/webhook"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	processor webhook.Processor
}

func NewOrderHandler(p webhook.Processor) *OrderHandler {
	return &OrderHandler{processor: p}
}

func (h *OrderHandler) Webhook(c *gin.Context) {
	var event order.PaymentWebhook
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}

	err := h.processor.ProcessOrderWebhook(c.Request.Context(), event)
	if err != nil {
		if errors.Is(err, order.ErrInvalidStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		} else if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		} else if errors.Is(err, order.ErrEventAlreadyStored) {
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.Status(http.StatusAccepted)
}
