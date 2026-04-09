package handlers

import (
	"TestTaskJustPay/services/ingest/apiclient"
	"TestTaskJustPay/services/ingest/dto"
	"TestTaskJustPay/services/ingest/webhook"
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
	var req dto.OrderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Missing order_id"})
		return
	}

	err := h.processor.ProcessOrderUpdate(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, apiclient.ErrInvalidStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		} else if errors.Is(err, apiclient.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		} else if errors.Is(err, apiclient.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.Status(http.StatusAccepted)
}
