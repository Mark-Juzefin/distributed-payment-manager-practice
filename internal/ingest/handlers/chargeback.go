package handlers

import (
	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/api/webhook"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChargebackHandler struct {
	processor webhook.Processor
}

func NewChargebackHandler(p webhook.Processor) *ChargebackHandler {
	return &ChargebackHandler{processor: p}
}

func (h *ChargebackHandler) Webhook(c *gin.Context) {
	var event dispute.ChargebackWebhook
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid payload"})
		return
	}

	err := h.processor.ProcessDisputeWebhook(c.Request.Context(), event)
	if err != nil {
		if errors.Is(err, order.ErrInvalidStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		} else if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		} else if errors.Is(err, dispute.ErrEventAlreadyStored) {
			c.JSON(http.StatusOK, gin.H{"message": "Duplicate webhook received"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.Status(http.StatusAccepted)
}
