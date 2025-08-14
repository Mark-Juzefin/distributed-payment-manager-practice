package dispute

import (
	"encoding/json"
	"time"
)

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
)

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
