package updates

import (
	"errors"
	"net/http"
	"time"

	"TestTaskJustPay/services/paymanager/dispute"
	"TestTaskJustPay/services/paymanager/order"
	"TestTaskJustPay/services/paymanager/payment"

	"github.com/gin-gonic/gin"
)

// orderUpdateRequest is the internal DTO for order webhook updates from Ingest service.
type orderUpdateRequest struct {
	ProviderEventID string            `json:"provider_event_id"`
	OrderID         string            `json:"order_id"`
	UserID          string            `json:"user_id"`
	Status          string            `json:"status"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CreatedAt       time.Time         `json:"created_at"`
	Meta            map[string]string `json:"meta,omitempty"`
}

type orderUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// disputeUpdateRequest is the internal DTO for dispute webhook updates from Ingest service.
type disputeUpdateRequest struct {
	ProviderEventID string            `json:"provider_event_id"`
	OrderID         string            `json:"order_id"`
	UserID          string            `json:"user_id"`
	Status          string            `json:"status"`
	Reason          string            `json:"reason"`
	Amount          float64           `json:"amount"`
	Currency        string            `json:"currency"`
	OccurredAt      time.Time         `json:"occurred_at"`
	EvidenceDueAt   *time.Time        `json:"evidence_due_at,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
}

type disputeUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type Handler struct {
	orderService   *order.OrderService
	disputeService *dispute.DisputeService
	paymentService *payment.PaymentService
}

func New(orderService *order.OrderService, disputeService *dispute.DisputeService, paymentService *payment.PaymentService) *Handler {
	return &Handler{
		orderService:   orderService,
		disputeService: disputeService,
		paymentService: paymentService,
	}
}

func (h *Handler) HandleOrderUpdate(c *gin.Context) {
	var req orderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, orderUpdateResponse{
			Success: false,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	webhook := order.OrderUpdate{
		ProviderEventID: req.ProviderEventID,
		OrderId:         req.OrderID,
		UserId:          req.UserID,
		Status:          order.Status(req.Status),
		UpdatedAt:       req.UpdatedAt,
		CreatedAt:       req.CreatedAt,
		Meta:            req.Meta,
	}

	err := h.orderService.ProcessOrderUpdate(c.Request.Context(), webhook)
	if err != nil {
		switch {
		case errors.Is(err, order.ErrNotFound):
			c.JSON(http.StatusNotFound, orderUpdateResponse{Success: false, Message: err.Error()})
		case errors.Is(err, order.ErrInvalidStatus):
			c.JSON(http.StatusUnprocessableEntity, orderUpdateResponse{Success: false, Message: err.Error()})
		case errors.Is(err, order.ErrEventAlreadyStored):
			c.JSON(http.StatusOK, orderUpdateResponse{Success: true, Message: "event already processed"})
		default:
			c.JSON(http.StatusInternalServerError, orderUpdateResponse{Success: false, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, orderUpdateResponse{Success: true})
}

func (h *Handler) HandleDisputeUpdate(c *gin.Context) {
	var req disputeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, disputeUpdateResponse{
			Success: false,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	webhook := dispute.ChargebackWebhook{
		ProviderEventID: req.ProviderEventID,
		OrderID:         req.OrderID,
		UserID:          req.UserID,
		Status:          dispute.ChargebackStatus(req.Status),
		Reason:          req.Reason,
		Money: dispute.Money{
			Amount:   req.Amount,
			Currency: req.Currency,
		},
		OccurredAt:    req.OccurredAt,
		EvidenceDueAt: req.EvidenceDueAt,
		Meta:          req.Meta,
	}

	err := h.disputeService.ProcessChargeback(c.Request.Context(), webhook)
	if err != nil {
		switch {
		case errors.Is(err, dispute.ErrEventAlreadyStored):
			c.JSON(http.StatusOK, disputeUpdateResponse{Success: true, Message: "event already processed"})
		default:
			c.JSON(http.StatusInternalServerError, disputeUpdateResponse{Success: false, Message: err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, disputeUpdateResponse{Success: true})
}

func (h *Handler) HandlePaymentWebhook(c *gin.Context) {
	var req payment.CaptureWebhook
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := h.paymentService.ProcessCaptureWebhook(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
