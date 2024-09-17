package auth

import (
	"encoding/base64"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

var TestPrincipal1 = &Principal{
	Organization: fhir.Organization{
		Name: to.Ptr("Test Organization 1"),
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  to.Ptr("1"),
			},
		},
		Address: []fhir.Address{
			{
				City: to.Ptr("Bugland"),
			},
		},
	},
}

var TestPrincipal2 = &Principal{
	Organization: fhir.Organization{
		Name: to.Ptr("Test Organization 2"),
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  to.Ptr("2"),
			},
		},
		Address: []fhir.Address{
			{
				City: to.Ptr("Testland"),
			},
		},
	},
}

// AuthenticatedTestRoundTripper returns a RoundTripper that adds a X-Userinfo header to the request
// with static user information. This is useful for testing purposes.
func AuthenticatedTestRoundTripper(underlying http.RoundTripper, principal *Principal, xSCPContext string) http.RoundTripper {
	if underlying == nil {
		underlying = http.DefaultTransport
	}

	if principal == nil {
		principal = TestPrincipal1
	}

	data, _ := json.Marshal(principal.Organization)
	bearerToken := base64.StdEncoding.EncodeToString(data)
	return headerDecoratorRoundTripper{
		inner: underlying,
		header: map[string]string{
			"Authorization": "Bearer " + bearerToken,
			"X-SCP-Context": xSCPContext,
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
