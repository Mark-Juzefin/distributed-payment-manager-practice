package transaction

import (
	"time"

	"github.com/google/uuid"
)

type RefundStatus string

const (
	RefundStatusPending RefundStatus = "refund_pending"
	RefundStatusDone    RefundStatus = "refunded"
	RefundStatusFailed  RefundStatus = "refund_failed"
)

type Refund struct {
	ID             uuid.UUID
	TransactionID  uuid.UUID
	Amount         int64
	Status         RefundStatus
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewRefundPending(txID uuid.UUID, amount int64, idempotencyKey string) *Refund {
	now := time.Now().UTC()
	return &Refund{
		ID:             uuid.New(),
		TransactionID:  txID,
		Amount:         amount,
		Status:         RefundStatusPending,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (r *Refund) MarkRefunded() {
	r.Status = RefundStatusDone
	r.UpdatedAt = time.Now().UTC()
}

func (r *Refund) MarkFailed() {
	r.Status = RefundStatusFailed
	r.UpdatedAt = time.Now().UTC()
}
