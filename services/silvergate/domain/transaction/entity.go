package transaction

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusAuthorized        Status = "authorized"
	StatusDeclined          Status = "declined"
	StatusCapturePending    Status = "capture_pending"
	StatusCaptured          Status = "captured"
	StatusCaptureFailed     Status = "capture_failed"
	StatusVoided            Status = "voided"
	StatusPartiallyRefunded Status = "partially_refunded"
	StatusRefunded          Status = "refunded"
)

type Transaction struct {
	ID             uuid.UUID
	MerchantID     string
	OrderRef       string
	Amount         int64
	Currency       string
	CardToken      string
	Status         Status
	DeclineReason  string
	IdempotencyKey string
	RefundedAmount int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewAuthorized(merchantID, orderRef string, amount int64, currency, cardToken string) *Transaction {
	now := time.Now().UTC()
	return &Transaction{
		ID:         uuid.New(),
		MerchantID: merchantID,
		OrderRef:   orderRef,
		Amount:     amount,
		Currency:   currency,
		CardToken:  cardToken,
		Status:     StatusAuthorized,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func NewDeclined(merchantID, orderRef string, amount int64, currency, cardToken, reason string) *Transaction {
	now := time.Now().UTC()
	return &Transaction{
		ID:            uuid.New(),
		MerchantID:    merchantID,
		OrderRef:      orderRef,
		Amount:        amount,
		Currency:      currency,
		CardToken:     cardToken,
		Status:        StatusDeclined,
		DeclineReason: reason,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func (t *Transaction) MarkCapturePending(idempotencyKey string) error {
	if t.Status != StatusAuthorized {
		return ErrInvalidTransition
	}
	t.Status = StatusCapturePending
	t.IdempotencyKey = idempotencyKey
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Transaction) MarkCaptured() error {
	if t.Status != StatusCapturePending {
		return ErrInvalidTransition
	}
	t.Status = StatusCaptured
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Transaction) MarkCaptureFailed() error {
	if t.Status != StatusCapturePending {
		return ErrInvalidTransition
	}
	t.Status = StatusCaptureFailed
	t.UpdatedAt = time.Now().UTC()
	return nil
}

func (t *Transaction) MarkVoided() error {
	if t.Status != StatusAuthorized {
		return ErrInvalidTransition
	}
	t.Status = StatusVoided
	t.UpdatedAt = time.Now().UTC()
	return nil
}

var validTransitions = map[Status][]Status{
	StatusAuthorized:        {StatusCapturePending, StatusVoided},
	StatusCapturePending:    {StatusCaptured, StatusCaptureFailed},
	StatusCaptured:          {StatusPartiallyRefunded, StatusRefunded},
	StatusPartiallyRefunded: {StatusPartiallyRefunded, StatusRefunded},
}

func (s Status) CanTransitionTo(target Status) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

var ErrInvalidTransition = errors.New("invalid status transition")
