package handlers

import (
	"net/http"

	"TestTaskJustPay/services/ingest/dto"
	"TestTaskJustPay/services/ingest/webhook"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	processor webhook.Processor
}

func NewPaymentHandler(p webhook.Processor) *PaymentHandler {
	return &PaymentHandler{processor: p}
}

func (h *PaymentHandler) Webhook(c *gin.Context) {
	var req dto.PaymentWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid payload"})
		return
	}

	if err := h.processor.ProcessPaymentWebhook(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.Status(http.StatusAccepted)
}
