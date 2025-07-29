package apperror

import "errors"

var ErrUnappropriatedStatus = errors.New("ErrUnappropriatedStatus")
var ErrOrderNotFound = errors.New("ErrOrderNotFound")
var ErrEventAlreadyStored = errors.New("ErrEventAlreadyStored")

var ErrInvalidOrdersQuery = errors.New("invalid orders query")
