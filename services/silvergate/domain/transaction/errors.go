package transaction

import "errors"

var (
	ErrNotFound             = errors.New("transaction not found")
	ErrAlreadyCaptured      = errors.New("transaction already captured")
	ErrDuplicateIdempotency = errors.New("duplicate idempotency key")
)
