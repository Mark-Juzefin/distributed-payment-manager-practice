package ordercontroller

import (
	"errors"
	"net/http"
	"time"

	"TestTaskJustPay/services/paymanager/internal/order"

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

// HandleUpdate processes an order webhook update forwarded from the Ingest service.
func (h *HTTPHandler) HandleUpdate(c *gin.Context) {
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

	err := h.service.ProcessOrderUpdate(c.Request.Context(), webhook)
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
