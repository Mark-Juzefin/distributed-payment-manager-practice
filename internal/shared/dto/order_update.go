package dto

import "time"

// OrderUpdateRequest represents an order update request from Ingest to API service.
// It mirrors order.PaymentWebhook but is decoupled from domain types.
type OrderUpdateRequest struct {
	ProviderEventID string            `json:"provider_event_id"`
	OrderID         string            `json:"order_id"`
	UserID          string            `json:"user_id"`
	Status          string            `json:"status"`
	UpdatedAt       time.Time         `json:"updated_at"`
	CreatedAt       time.Time         `json:"created_at"`
	Meta            map[string]string `json:"meta,omitempty"`
}

// OrderUpdateResponse represents the response from API service.
type OrderUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
