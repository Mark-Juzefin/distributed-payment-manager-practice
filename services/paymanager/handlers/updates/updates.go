package updates

import (
	"errors"
	"net/http"

	"TestTaskJustPay/services/paymanager/domain/dispute"
	"TestTaskJustPay/services/paymanager/domain/order"
	"TestTaskJustPay/services/paymanager/domain/payment"
	"TestTaskJustPay/services/paymanager/dto"

	"github.com/gin-gonic/gin"
)

// UpdatesHandler handles internal service-to-service update requests.
type UpdatesHandler struct {
	orderService   *order.OrderService
	disputeService *dispute.DisputeService
	paymentService *payment.PaymentService
}

func NewUpdatesHandler(orderService *order.OrderService, disputeService *dispute.DisputeService, paymentService *payment.PaymentService) *UpdatesHandler {
	return &UpdatesHandler{
		orderService:   orderService,
		disputeService: disputeService,
		paymentService: paymentService,
	}
}

// HandleOrderUpdate processes order webhook updates from Ingest service.
// POST /internal/updates/orders
func (h *UpdatesHandler) HandleOrderUpdate(c *gin.Context) {
	var req dto.OrderUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.OrderUpdateResponse{
			Success: false,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	webhook := mapOrderUpdateToWebhook(req)

	err := h.orderService.ProcessOrderUpdate(c.Request.Context(), webhook)
	if err != nil {
		h.handleOrderError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.OrderUpdateResponse{
		Success: true,
	})
}

// HandleDisputeUpdate processes dispute/chargeback webhook updates from Ingest service.
// POST /internal/updates/disputes
func (h *UpdatesHandler) HandleDisputeUpdate(c *gin.Context) {
	var req dto.DisputeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.DisputeUpdateResponse{
			Success: false,
			Message: "invalid request body: " + err.Error(),
		})
		return
	}

	webhook := mapDisputeUpdateToWebhook(req)

	err := h.disputeService.ProcessChargeback(c.Request.Context(), webhook)
	if err != nil {
		h.handleDisputeError(c, err)
		return
	}

	c.JSON(http.StatusOK, dto.DisputeUpdateResponse{
		Success: true,
	})
}

func (h *UpdatesHandler) handleOrderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, order.ErrNotFound):
		c.JSON(http.StatusNotFound, dto.OrderUpdateResponse{
			Success: false,
			Message: err.Error(),
		})
	case errors.Is(err, order.ErrInvalidStatus):
		c.JSON(http.StatusUnprocessableEntity, dto.OrderUpdateResponse{
			Success: false,
			Message: err.Error(),
		})
	case errors.Is(err, order.ErrEventAlreadyStored):
		// Idempotent - treat as success
		c.JSON(http.StatusOK, dto.OrderUpdateResponse{
			Success: true,
			Message: "event already processed",
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.OrderUpdateResponse{
			Success: false,
			Message: err.Error(),
		})
	}
}

func (h *UpdatesHandler) handleDisputeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, dispute.ErrEventAlreadyStored):
		// Idempotent - treat as success
		c.JSON(http.StatusOK, dto.DisputeUpdateResponse{
			Success: true,
			Message: "event already processed",
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.DisputeUpdateResponse{
			Success: false,
			Message: err.Error(),
		})
	}
}

// HandlePaymentWebhook processes payment webhook updates from Ingest service.
// POST /internal/updates/payments
func (h *UpdatesHandler) HandlePaymentWebhook(c *gin.Context) {
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

func mapOrderUpdateToWebhook(req dto.OrderUpdateRequest) order.OrderUpdate {
	return order.OrderUpdate{
		ProviderEventID: req.ProviderEventID,
		OrderId:         req.OrderID,
		UserId:          req.UserID,
		Status:          order.Status(req.Status),
		UpdatedAt:       req.UpdatedAt,
		CreatedAt:       req.CreatedAt,
		Meta:            req.Meta,
	}
}

func mapDisputeUpdateToWebhook(req dto.DisputeUpdateRequest) dispute.ChargebackWebhook {
	return dispute.ChargebackWebhook{
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
}
