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

// Value returns the value of a pointer or the zero value of the type if the pointer is nil.
func Value[T any](v *T) T {
	if v == nil {
		value := reflect.New(reflect.TypeOf(v).Elem())
		return value.Elem().Interface().(T)
	}
	return *v
}
