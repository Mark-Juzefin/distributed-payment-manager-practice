package dispute

import (
	"encoding/json"
	"time"
)

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

type DisputeEventQuery struct {
	DisputeIDs []string
	Kinds      []DisputeEventKind
}

type DisputeEventQueryBuilder struct {
	query *DisputeEventQuery
}

func NewDisputeEventQueryBuilder() *DisputeEventQueryBuilder {
	return &DisputeEventQueryBuilder{
		query: &DisputeEventQuery{},
	}
}

func (b *DisputeEventQueryBuilder) WithDisputeIDs(disputeIDs ...string) *DisputeEventQueryBuilder {
	b.query.DisputeIDs = disputeIDs
	return b
}

func (b *DisputeEventQueryBuilder) WithKinds(kinds ...DisputeEventKind) *DisputeEventQueryBuilder {
	b.query.Kinds = kinds
	return b
}

func (b *DisputeEventQueryBuilder) Build() *DisputeEventQuery {
	return b.query
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
