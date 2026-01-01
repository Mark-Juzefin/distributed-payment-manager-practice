package dispute

import "errors"

var (
	// ErrAlreadyExists is returned when attempting to create a dispute that already exists
	ErrAlreadyExists = errors.New("dispute already exists")

	// ErrEventAlreadyStored is returned when event with same (dispute_id, provider_event_id) already exists
	ErrEventAlreadyStored = errors.New("event already stored")
)
