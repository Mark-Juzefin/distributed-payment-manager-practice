package product

import "errors"

var (
	ErrNotFound      = errors.New("product not found")
	ErrSlugConflict  = errors.New("slug already exists for this merchant")
	ErrInvalidSlug   = errors.New("invalid slug format")
	ErrSlugRemoval   = errors.New("slug cannot be removed after creation")
	ErrFieldsLocked  = errors.New("pricing fields are locked after first purchase")
	ErrArchived      = errors.New("cannot modify archived product")
	ErrEmptyUpdate   = errors.New("update has no fields to change")
	ErrLimitTooLarge = errors.New("list limit exceeds maximum")
)
