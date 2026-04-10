package transaction

import "errors"

var (
	ErrNotFound             = errors.New("transaction not found")
	ErrAlreadyCaptured      = errors.New("transaction already captured")
	ErrDuplicateIdempotency = errors.New("duplicate idempotency key")
	ErrRefundExceedsAmount  = errors.New("refund amount exceeds remaining balance")
	ErrNotRefundable        = errors.New("transaction is not in a refundable state")
)
