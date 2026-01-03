package handlers

import (
	"TestTaskJustPay/internal/shared/domain/dispute"
	"TestTaskJustPay/internal/shared/domain/order"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChargebackHandler struct {
	service *dispute.DisputeService
}

func NewChargebackHandler(s *dispute.DisputeService) *ChargebackHandler {
	return &ChargebackHandler{service: s}
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
