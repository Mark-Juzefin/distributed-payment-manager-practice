package handlers

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/domain/transaction"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type voidRequest struct {
	TransactionID string `json:"transaction_id" binding:"required"`
}

type voidResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

type VoidHandler struct {
	svc *transaction.Service
}

func NewVoidHandler(svc *transaction.Service) *VoidHandler {
	return &VoidHandler{svc: svc}
}

func (h *VoidHandler) Handle(c *gin.Context) {
	var req voidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction_id"})
		return
	}

	result, err := h.svc.Void(c.Request.Context(), txID)
	if err != nil {
		switch {
		case errors.Is(err, transaction.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		case errors.Is(err, transaction.ErrInvalidTransition):
			c.JSON(http.StatusConflict, gin.H{"error": "transaction cannot be voided in current state"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "void failed"})
		}
		return
	}

	c.JSON(http.StatusOK, voidResponse{
		TransactionID: result.TransactionID.String(),
		Status:        string(result.Status),
	})
}
