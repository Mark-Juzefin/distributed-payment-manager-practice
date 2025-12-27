package dispute

import (
	"context"
	"encoding/json"
	"time"
)

type EventSink interface {
	// CreateDisputeEvent creates a new dispute event.
	// Returns apperror.ErrEventAlreadyStored if event with same (dispute_id, provider_event_id) already exists.
	CreateDisputeEvent(ctx context.Context, event NewDisputeEvent) (*DisputeEvent, error)
	GetDisputeEvents(ctx context.Context, query DisputeEventQuery) (DisputeEventPage, error)
}

type DisputeEvent struct {
	EventID string `json:"event_id"`
	NewDisputeEvent
}

type NewDisputeEvent struct {
	DisputeID       string           `json:"dispute_id"`
	Kind            DisputeEventKind `json:"kind"`
	ProviderEventID string           `json:"provider_event_id"`
	Data            json.RawMessage  `json:"data"`
	CreatedAt       time.Time        `json:"created_at"`
}

type DisputeEventKind string

const (
	DisputeEventWebhookOpened     DisputeEventKind = "webhook_opened"
	DisputeEventWebhookUpdated    DisputeEventKind = "webhook_updated"
	DisputeEventProviderDecision  DisputeEventKind = "provider_decision"
	DisputeEventEvidenceSubmitted DisputeEventKind = "evidence_submitted"
	DisputeEventEvidenceAdded     DisputeEventKind = "evidence_added"
)

type DisputeEventPage struct {
	Items      []DisputeEvent `json:"items"`
	NextCursor string         `json:"next_cursor"`
	HasMore    bool           `json:"has_more"`
}

type DisputeEventQuery struct {
	DisputeIDs []string           `json:"dispute_ids" url:"dispute_ids" form:"dispute_ids,omitempty"`
	Kinds      []DisputeEventKind `json:"kinds" url:"kinds" form:"kinds,omitempty"`

	TimeFrom *time.Time `json:"time_from,omitempty" url:"time_from,omitempty" form:"time_from,omitempty"`
	TimeTo   *time.Time `json:"time_to,omitempty" url:"time_to,omitempty" form:"time_to,omitempty"`

	Limit   int    `json:"limit" url:"limit" form:"limit"`
	Cursor  string `json:"cursor" url:"cursor" form:"cursor"`
	SortAsc bool   `json:"sort_asc" url:"sort_asc" form:"sort_asc"`
}

func deriveKindFromChargebackStatus(status ChargebackStatus) DisputeEventKind {
	var kind DisputeEventKind
	switch status {
	case ChargebackOpened:
		kind = DisputeEventWebhookOpened
	case ChargebackUpdated:
		kind = DisputeEventWebhookUpdated
	case ChargebackClosed:
		kind = DisputeEventProviderDecision
	}
	return kind
}
