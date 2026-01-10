package apiclient

import "errors"

var (
	// ErrNotFound is returned when the resource is not found (HTTP 404)
	ErrNotFound = errors.New("resource not found")

	// ErrConflict is returned when there's a conflict (HTTP 409)
	ErrConflict = errors.New("conflict")

	// ErrInvalidStatus is returned when the status transition is invalid (HTTP 422)
	ErrInvalidStatus = errors.New("invalid status transition")

	// ErrServiceUnavailable is returned when the API service is unavailable (HTTP 5xx, timeout)
	ErrServiceUnavailable = errors.New("api service unavailable")

	// ErrBadRequest is returned when the request is malformed (HTTP 400)
	ErrBadRequest = errors.New("bad request")
)
