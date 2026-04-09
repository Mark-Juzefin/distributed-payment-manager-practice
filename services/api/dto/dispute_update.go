package dto

import "time"

// DisputeUpdateRequest represents a dispute/chargeback update request from Ingest to API service.
// It mirrors dispute.ChargebackWebhook but is decoupled from domain types.
type DisputeUpdateRequest struct {
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

// DisputeUpdateResponse represents the response from API service.
type DisputeUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
