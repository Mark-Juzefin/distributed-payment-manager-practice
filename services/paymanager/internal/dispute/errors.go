package dispute

import "errors"

var (
	ErrAlreadyExists      = errors.New("dispute already exists")
	ErrEventAlreadyStored = errors.New("event already stored")
)
