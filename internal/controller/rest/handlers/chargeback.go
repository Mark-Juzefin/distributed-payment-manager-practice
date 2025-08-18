package handlers

import (
	"TestTaskJustPay/internal/controller/apperror"
	"TestTaskJustPay/internal/domain/dispute"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChargebackHandler struct {
	service *dispute.DisputeService
}

func NewChargebackHandler(s *dispute.DisputeService) ChargebackHandler {
	return ChargebackHandler{service: s}
}

func (h *ChargebackHandler) Webhook(c *gin.Context) {
	var event dispute.ChargebackWebhook
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid payload"})
		return
	}

	err := h.service.ProcessChargeback(c, event)
	if err != nil {
		if errors.Is(err, apperror.ErrUnappropriatedStatus) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		} else if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		} else if errors.Is(err, apperror.ErrEventAlreadyStored) {
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
		if errors.Is(err, apperror.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "Dispute not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, d)
}
