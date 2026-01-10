package updates

import (
	"errors"
	"net/http"

	"TestTaskJustPay/internal/api/domain/dispute"
	"TestTaskJustPay/internal/api/domain/order"
	"TestTaskJustPay/internal/shared/dto"

	"github.com/gin-gonic/gin"
)

// UpdatesHandler handles internal service-to-service update requests.
type UpdatesHandler struct {
	orderService   *order.OrderService
	disputeService *dispute.DisputeService
}

func NewUpdatesHandler(orderService *order.OrderService, disputeService *dispute.DisputeService) *UpdatesHandler {
	return &UpdatesHandler{
		orderService:   orderService,
		disputeService: disputeService,
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

	err := h.orderService.ProcessPaymentWebhook(c.Request.Context(), webhook)
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

func mapOrderUpdateToWebhook(req dto.OrderUpdateRequest) order.PaymentWebhook {
	return order.PaymentWebhook{
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
