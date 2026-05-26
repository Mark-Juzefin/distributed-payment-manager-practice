package transaction

import (
	"context"

	"github.com/google/uuid"
)

// Repo is the persistence contract for transactions and refunds.
type Repo interface {
	Create(ctx context.Context, tx *Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*Transaction, error)
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (*Transaction, error)
	// GetByPurchaseIdempotencyKey is the pre-check lookup for /purchase idempotency.
	// Returns ErrNotFound when no transaction exists for the (merchant_id, key) pair.
	GetByPurchaseIdempotencyKey(ctx context.Context, merchantID, key string) (*Transaction, error)
	UpdateStatus(ctx context.Context, tx *Transaction) error
	CompareAndUpdateStatus(ctx context.Context, id uuid.UUID, expected, next Status) error
	UpdateRefund(ctx context.Context, tx *Transaction) error
	CreateRefund(ctx context.Context, refund *Refund) error
	UpdateRefundStatus(ctx context.Context, refund *Refund) error
	ReleaseRefundAmount(ctx context.Context, txID uuid.UUID, amount int64) error
}

// WebhookSender notifies the merchant of transaction lifecycle events.
type WebhookSender interface {
	SendCaptureResult(ctx context.Context, tx *Transaction) error
	SendRefundResult(ctx context.Context, tx *Transaction, refund *Refund) error
}
