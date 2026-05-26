package purchasecontroller

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/silvergate/internal/merchantauth"
	"TestTaskJustPay/services/silvergate/internal/product"
	"TestTaskJustPay/services/silvergate/internal/purchase"
	"TestTaskJustPay/services/silvergate/internal/transaction"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const idempotencyHeader = "Idempotency-Key"

type purchaseRequest struct {
	OrderID   string `json:"order_id" binding:"required"`
	ProductID string `json:"product_id" binding:"required,uuid"`
	CardToken string `json:"card_token" binding:"required"`
}

type purchaseResponse struct {
	TransactionID string `json:"transaction_id"`
	ProductID     string `json:"product_id"`
	OrderID       string `json:"order_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount,omitempty"`
	Currency      string `json:"currency,omitempty"`
	DeclineReason string `json:"decline_reason,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
	// TransactionID populated on partial-success errors (purchase_partially_persisted)
	// so the caller can manually capture or void.
	TransactionID string `json:"transaction_id,omitempty"`
}

// Handler invokes purchase.Service.Purchase and maps the result onto HTTP per
// spec §Error responses.
type Handler struct {
	svc *purchase.Service
}

func NewHandler(svc *purchase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Handle(c *gin.Context) {
	merchantID, ok := merchantauth.FromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "merchant context missing"})
		return
	}

	idempotencyKey := c.GetHeader(idempotencyHeader)
	if idempotencyKey == "" {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "missing Idempotency-Key header", Code: "missing_idempotency_key"})
		return
	}

	var req purchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: err.Error(), Code: "invalid_request"})
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid product_id", Code: "invalid_request"})
		return
	}

	resp, err := h.svc.Purchase(c.Request.Context(), purchase.Request{
		MerchantID:     merchantID,
		OrderID:        req.OrderID,
		ProductID:      productID,
		CardToken:      req.CardToken,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		writeError(c, resp, err)
		return
	}

	c.JSON(http.StatusOK, toResponse(resp))
}

func writeError(c *gin.Context, partial purchase.Response, err error) {
	switch {
	case errors.Is(err, product.ErrNotFound):
		c.JSON(http.StatusNotFound, errorResponse{Error: "product not found", Code: "product_not_found"})
	case errors.Is(err, purchase.ErrProductArchived):
		c.JSON(http.StatusUnprocessableEntity, errorResponse{Error: "product is archived", Code: "product_archived"})
	case errors.Is(err, purchase.ErrIdempotencyConflict):
		c.JSON(http.StatusConflict, errorResponse{Error: "idempotency key reused with different request", Code: "idempotency_conflict"})
	case errors.Is(err, purchase.ErrCapturePartiallyApplied):
		c.JSON(http.StatusInternalServerError, errorResponse{
			Error:         "authorize succeeded but capture failed; manual recovery required",
			Code:          "purchase_partially_persisted",
			TransactionID: partial.TransactionID.String(),
		})
	default:
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal error", Code: "internal_error"})
	}
}

func toResponse(r purchase.Response) purchaseResponse {
	out := purchaseResponse{
		TransactionID: r.TransactionID.String(),
		ProductID:     r.ProductID.String(),
		OrderID:       r.OrderID,
		Status:        string(r.Status),
	}
	if r.Status == transaction.StatusCapturePending {
		out.Amount = r.Amount
		out.Currency = r.Currency
	}
	if r.Status == transaction.StatusDeclined {
		out.DeclineReason = r.DeclineReason
	}
	return out
}
