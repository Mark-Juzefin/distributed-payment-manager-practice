package handlers

import (
	"TestTaskJustPay/services/ingest/apiclient"
	"TestTaskJustPay/services/ingest/dto"
	"TestTaskJustPay/services/ingest/webhook"
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
	var req dto.DisputeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid payload"})
		return
	}

	err := h.processor.ProcessDisputeUpdate(c.Request.Context(), req)
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
