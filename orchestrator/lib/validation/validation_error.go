package validation

type Error struct {
	Code string
}

func (e *Error) Error() string {
	return e.Code
}
