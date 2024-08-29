package auth

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

// AuthenticatedTestRoundTripper returns a RoundTripper that adds a X-Userinfo header to the request
// with static user information. This is useful for testing purposes.
func AuthenticatedTestRoundTripper(underlying http.RoundTripper) http.RoundTripper {
	if underlying == nil {
		underlying = http.DefaultTransport
	}
	return headerDecoratorRoundTripper{
		inner: underlying,
		header: map[string]string{
			"Authorization": "Bearer valid",
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

func TestPrincipal(ctx context.Context) context.Context {
	return context.WithValue(ctx, principalContextKey, Principal{
		Organization: fhir.Organization{
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
		},
	})
}
