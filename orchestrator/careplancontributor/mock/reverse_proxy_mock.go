package mock

import (
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
)

// MockReverseProxy is a mock implementation of a ReverseProxy for unit tests.
type MockReverseProxy struct {
	Proxy coolfhir.HttpProxy
}

func NewMockReverseProxy(target *url.URL, orcaPublicUrl *url.URL, transport http.RoundTripper, allowCaching bool) (*MockReverseProxy, error) {
	proxy := coolfhir.NewProxy(
		"MockProxy",
		log.Logger,
		target,
		"/cpc/cps/fhir",
		orcaPublicUrl.JoinPath("/cpc/cps/fhir"),
		transport,
		allowCaching,
	)

	return &MockReverseProxy{Proxy: proxy}, nil
}

// ServeHTTP handles the HTTP request and forwards it to the target.
func (m *MockReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Proxy.ServeHTTP(w, r)
}
