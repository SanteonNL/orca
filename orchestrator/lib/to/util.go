package to

import "reflect"

func Ptr[T any](v T) *T {
	return &v
}

func NilString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func EmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// Empty returns the zero value of the type if the pointer is nil, otherwise it returns the value pointed to by the pointer.
func Empty[T any](ptr *T) T {
	if ptr == nil {
		var zero T
		return zero
	}
	return *ptr
}

// Value returns the value of a pointer or the zero value of the type if the pointer is nil.
func Value[T any](v *T) T {
	if v == nil {
		value := reflect.New(reflect.TypeOf(v).Elem())
		return value.Elem().Interface().(T)
	}
	return *v
}
