package pointers

// Ptr returns the pointer to the input parameter
func Ptr[T any](v T) *T {
	return &v
}
