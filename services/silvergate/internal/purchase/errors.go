package purchase

import "errors"

var (
	ErrProductArchived         = errors.New("product is archived")
	ErrIdempotencyConflict     = errors.New("idempotency key reused with different request body")
	ErrCapturePartiallyApplied = errors.New("authorize succeeded but capture failed; manual recovery required")
)
