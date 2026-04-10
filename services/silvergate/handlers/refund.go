package handlers

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/domain/transaction"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type refundRequest struct {
	TransactionID  string `json:"transaction_id" binding:"required"`
	Amount         int64  `json:"amount" binding:"required,min=1"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

type refundResponse struct {
	RefundID      string `json:"refund_id"`
	TransactionID string `json:"transaction_id"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
}

type RefundHandler struct {
	svc *transaction.Service
}

func NewRefundHandler(svc *transaction.Service) *RefundHandler {
	return &RefundHandler{svc: svc}
}

func (h *RefundHandler) Handle(c *gin.Context) {
	var req refundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction_id"})
		return
	}

	result, err := h.svc.Refund(c.Request.Context(), transaction.RefundRequest{
		TransactionID:  txID,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, transaction.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		case errors.Is(err, transaction.ErrNotRefundable):
			c.JSON(http.StatusConflict, gin.H{"error": "transaction is not in a refundable state"})
		case errors.Is(err, transaction.ErrRefundExceedsAmount):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "refund amount exceeds remaining balance"})
		case errors.Is(err, transaction.ErrDuplicateIdempotency):
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate idempotency key"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "refund failed"})
		}
		return
	}

	c.JSON(http.StatusAccepted, refundResponse{
		RefundID:      result.RefundID.String(),
		TransactionID: result.TransactionID.String(),
		Amount:        result.Amount,
		Status:        string(result.Status),
	})
}
