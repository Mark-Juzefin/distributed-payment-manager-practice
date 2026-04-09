package payment

import "errors"

var (
	ErrNotFound      = errors.New("payment not found")
	ErrAlreadyExists = errors.New("payment already exists")
	ErrInvalidStatus = errors.New("invalid payment status transition")
)
