package disputecontroller

import (
	"errors"
	"net/http"
	"time"

	"TestTaskJustPay/services/paymanager/internal/dispute"

	"github.com/gin-gonic/gin"
)

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

// HandleUpdate processes a dispute webhook update forwarded from the Ingest service.
func (h *HTTPHandler) HandleUpdate(c *gin.Context) {
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

	err := h.service.ProcessChargeback(c.Request.Context(), webhook)
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
