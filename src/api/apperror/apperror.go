package apperror

import "errors"

var UnappropriatedStatus = errors.New("UnappropriatedStatus")
var OrderNotFound = errors.New("OrderNotFound")
var EventAlreadyStored = errors.New("EventAlreadyStored")
