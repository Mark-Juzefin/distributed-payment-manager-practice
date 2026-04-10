package handlers

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/domain/transaction"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type captureRequest struct {
	TransactionID  string `json:"transaction_id" binding:"required"`
	Amount         int64  `json:"amount" binding:"required,min=1"`
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

type captureResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
}

type CaptureHandler struct {
	svc *transaction.Service
}

func NewCaptureHandler(svc *transaction.Service) *CaptureHandler {
	return &CaptureHandler{svc: svc}
}

func (h *CaptureHandler) Handle(c *gin.Context) {
	var req captureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txID, err := uuid.Parse(req.TransactionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid transaction_id"})
		return
	}

	result, err := h.svc.Capture(c.Request.Context(), transaction.CaptureRequest{
		TransactionID:  txID,
		Amount:         req.Amount,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		switch {
		case errors.Is(err, transaction.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		case errors.Is(err, transaction.ErrInvalidTransition):
			c.JSON(http.StatusConflict, gin.H{"error": "transaction cannot be captured in current state"})
		case errors.Is(err, transaction.ErrDuplicateIdempotency):
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate idempotency key"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "capture failed"})
		}
		return
	}

	c.JSON(http.StatusAccepted, captureResponse{
		TransactionID: result.TransactionID.String(),
		Status:        string(result.Status),
	})
}
