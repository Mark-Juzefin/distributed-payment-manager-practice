package dispute

import (
	"context"

	"TestTaskJustPay/services/paymanager/internal/gateway"
)

// DisputeRepo is the persistence contract for disputes.
type DisputeRepo interface {
	GetDisputes(ctx context.Context) ([]Dispute, error)
	GetDisputeByID(ctx context.Context, disputeID string) (*Dispute, error)
	GetDisputeByOrderID(ctx context.Context, orderID string) (*Dispute, error)

	CreateDispute(ctx context.Context, dispute NewDispute) (*Dispute, error)
	UpdateDispute(ctx context.Context, dispute Dispute) error

	UpsertEvidence(ctx context.Context, disputeID string, upsert EvidenceUpsert) (*Evidence, error)
	GetEvidence(ctx context.Context, disputeID string) (*Evidence, error)
}

// DisputeEvents is the event-sink contract for dispute domain events.
// TODO: remove in favor of direct eventstore.Store calls.
type DisputeEvents interface {
	CreateDisputeEvent(ctx context.Context, event NewDisputeEvent) (*DisputeEvent, error)
	GetDisputeEvents(ctx context.Context, query DisputeEventQuery) (DisputeEventPage, error)
}

// Provider is the minimal interface this domain requires from the payment gateway.
type Provider interface {
	SubmitRepresentment(ctx context.Context, req gateway.RepresentmentRequest) (gateway.RepresentmentResult, error)
}
