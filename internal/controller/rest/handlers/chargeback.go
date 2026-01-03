package handlers

import (
	"TestTaskJustPay/internal/domain/dispute"
	"TestTaskJustPay/internal/domain/order"
	"TestTaskJustPay/internal/webhook"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChargebackHandler struct {
	service   *dispute.DisputeService
	processor webhook.Processor
}

func NewChargebackHandler(s *dispute.DisputeService, processor webhook.Processor) ChargebackHandler {
	return ChargebackHandler{service: s, processor: processor}
}

func (h *ChargebackHandler) Webhook(c *gin.Context) {
	// Check if processor is available (nil in API kafka mode)
	if h.processor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "Webhook endpoint not available in this mode"})
		return
	}

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

func (h *ChargebackHandler) GetDispute(c *gin.Context) {
	disputeID := c.Param("dispute_id")
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing dispute_id"})
		return
	}

	d, err := h.service.GetDisputeByID(c, disputeID)
	if err != nil {
		if errors.Is(err, order.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Dispute not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, d)
}
