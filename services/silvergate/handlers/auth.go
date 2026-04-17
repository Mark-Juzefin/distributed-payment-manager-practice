package handlers

import (
	"net/http"

	"TestTaskJustPay/services/silvergate/domain/transaction"

	"github.com/gin-gonic/gin"
)

type authRequest struct {
	MerchantID string `json:"merchant_id" binding:"required"`
	OrderID    string `json:"order_id" binding:"required"`
	Amount     int64  `json:"amount" binding:"required,min=1"`
	Currency   string `json:"currency" binding:"required,len=3"`
	CardToken  string `json:"card_token" binding:"required"`
}

type authResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	OrderID       string `json:"order_id"`
	DeclineReason string `json:"decline_reason,omitempty"`
}

type AuthHandler struct {
	svc *transaction.Service
}

func NewAuthHandler(svc *transaction.Service) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Handle(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.svc.Authorize(c.Request.Context(), transaction.AuthRequest{
		MerchantID: req.MerchantID,
		OrderID:    req.OrderID,
		Amount:     req.Amount,
		Currency:   req.Currency,
		CardToken:  req.CardToken,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
		return
	}

	c.JSON(http.StatusOK, authResponse{
		TransactionID: result.TransactionID.String(),
		Status:        string(result.Status),
		OrderID:       req.OrderID,
		DeclineReason: result.DeclineReason,
	})
}
