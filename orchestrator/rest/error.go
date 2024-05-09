package rest

import "fmt"

var _ error = RemoteAPIError{}

// RemoteAPIError is returned when the remote API returned an error.
type RemoteAPIError struct {
	Err    error
	Result interface{}
}

func (r RemoteAPIError) Error() string {
	return fmt.Sprintf("remote API error: %v", r.Err)
}
