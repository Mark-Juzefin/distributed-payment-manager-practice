package order

import "errors"

var (
	// ErrNotFound is returned when order is not found
	ErrNotFound = errors.New("order not found")

	// ErrAlreadyExists is returned when attempting to create an order that already exists
	ErrAlreadyExists = errors.New("order already exists")

	// ErrInvalidStatus is returned when attempting an inappropriate status transition
	ErrInvalidStatus = errors.New("invalid status transition")

	// ErrOnHold is returned when attempting to capture an order that is on hold
	ErrOnHold = errors.New("order is on hold")

	// ErrInFinalStatus is returned when attempting to modify an order in final status
	ErrInFinalStatus = errors.New("order is in final status")

	// ErrInvalidQuery is returned when order query validation fails
	ErrInvalidQuery = errors.New("invalid orders query")

	// ErrEventAlreadyStored is returned when event with same (order_id, provider_event_id) already exists
	ErrEventAlreadyStored = errors.New("event already stored")
)
