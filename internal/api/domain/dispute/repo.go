package dispute

import (
	"context"
)

//go:generate mockgen -source repo.go -destination mock_repo.go -package dispute

type DisputeRepo interface {
	TxDisputeRepo
	InTransaction(ctx context.Context, fn func(repo TxDisputeRepo) error) error
}

type TxDisputeRepo interface {
	GetDisputes(ctx context.Context) ([]Dispute, error)
	GetDisputeByID(ctx context.Context, disputeID string) (*Dispute, error)
	GetDisputeByOrderID(ctx context.Context, orderID string) (*Dispute, error)

	CreateDispute(ctx context.Context, dispute NewDispute) (*Dispute, error)
	UpdateDispute(ctx context.Context, dispute Dispute) error

	UpsertEvidence(ctx context.Context, disputeID string, upsert EvidenceUpsert) (*Evidence, error)
	GetEvidence(ctx context.Context, disputeID string) (*Evidence, error)
}
