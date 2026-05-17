package order

import "errors"

var (
	ErrNotFound           = errors.New("order not found")
	ErrAlreadyExists      = errors.New("order already exists")
	ErrInvalidStatus      = errors.New("invalid status transition")
	ErrOnHold             = errors.New("order is on hold")
	ErrInFinalStatus      = errors.New("order is in final status")
	ErrInvalidQuery       = errors.New("invalid orders query")
	ErrEventAlreadyStored = errors.New("event already stored")
)
