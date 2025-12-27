package order

import (
	"context"
	"encoding/json"
	"time"
)

type EventSink interface {
	// CreateOrderEvent creates a new order event.
	// Returns apperror.ErrEventAlreadyStored if event with same (order_id, provider_event_id) already exists.
	CreateOrderEvent(ctx context.Context, event NewOrderEvent) (*OrderEvent, error)
	GetOrderEvents(ctx context.Context, query OrderEventQuery) (OrderEventPage, error)
}

type OrderEvent struct {
	EventID string `json:"event_id"`
	NewOrderEvent
}

type NewOrderEvent struct {
	OrderID         string          `json:"order_id"`
	Kind            OrderEventKind  `json:"kind"`
	ProviderEventID string          `json:"provider_event_id"`
	Data            json.RawMessage `json:"data"`
	CreatedAt       time.Time       `json:"created_at"`
}

type OrderEventKind string

const (
	OrderEventWebhookReceived  OrderEventKind = "webhook_received"
	OrderEventHoldSet          OrderEventKind = "hold_set"
	OrderEventHoldCleared      OrderEventKind = "hold_cleared"
	OrderEventCaptureRequested OrderEventKind = "capture_requested"
	OrderEventCaptureCompleted OrderEventKind = "capture_completed"
	OrderEventCaptureFailed    OrderEventKind = "capture_failed"
)

type OrderEventPage struct {
	Items      []OrderEvent `json:"items"`
	NextCursor string       `json:"next_cursor"`
	HasMore    bool         `json:"has_more"`
}

type OrderEventQuery struct {
	OrderIDs []string         `json:"order_ids" url:"order_ids" form:"order_ids,omitempty"`
	Kinds    []OrderEventKind `json:"kinds" url:"kinds" form:"kinds,omitempty"`

	TimeFrom *time.Time `json:"time_from,omitempty" url:"time_from,omitempty" form:"time_from,omitempty"`
	TimeTo   *time.Time `json:"time_to,omitempty" url:"time_to,omitempty" form:"time_to,omitempty"`

	Limit   int    `json:"limit" url:"limit" form:"limit"`
	Cursor  string `json:"cursor" url:"cursor" form:"cursor"`
	SortAsc bool   `json:"sort_asc" url:"sort_asc" form:"sort_asc"`
}
