package validation

type Validator[T any] interface {
	Validate(t T) error
}
