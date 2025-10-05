package utils

func NewPtr[T any](value T) *T {
	var newValue T = value
	return &newValue
}

func NewPtrp[T any](value *T) *T {
	if value == nil {
		return nil
	}
	var newValue = *value
	return &newValue
}
