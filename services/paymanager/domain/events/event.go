package events

import (
	"context"
	"encoding/json"
	"time"
)

type AggregateType string

const (
	AggregateOrder   AggregateType = "order"
	AggregateDispute AggregateType = "dispute"
)

type NewEvent struct {
	AggregateType  AggregateType
	AggregateID    string
	EventType      string // reuse existing kind strings: "webhook_received", "hold_set", etc.
	IdempotencyKey string // provider_event_id or generated UUID
	Payload        json.RawMessage
	CreatedAt      time.Time
}

type Event struct {
	ID string
	NewEvent
}

type Store interface {
	CreateEvent(ctx context.Context, event NewEvent) (*Event, error)
}
