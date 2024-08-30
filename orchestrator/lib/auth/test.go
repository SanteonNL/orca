package auth

import (
	"net/http"
)

// AuthenticatedTestRoundTripper returns a RoundTripper that adds a X-Userinfo header to the request
// with static user information. This is useful for testing purposes.
func AuthenticatedTestRoundTripper(underlying http.RoundTripper, bearerToken string) http.RoundTripper {
	if underlying == nil {
		underlying = http.DefaultTransport
	}

	if bearerToken == "" {
		bearerToken = "Bearer valid"
	} else {
		bearerToken = "Bearer " + bearerToken
	}
	return headerDecoratorRoundTripper{
		inner: underlying,
		header: map[string]string{
			"Authorization": bearerToken,
		},
	}
}

var _ http.RoundTripper = headerDecoratorRoundTripper{}

type headerDecoratorRoundTripper struct {
	inner  http.RoundTripper
	header map[string]string
}

func (h headerDecoratorRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	for name, value := range h.header {
		request.Header.Set(name, value)
	}
	return h.inner.RoundTrip(request)
}
