package eventstore

import "errors"

var ErrEventAlreadyStored = errors.New("event already stored")
