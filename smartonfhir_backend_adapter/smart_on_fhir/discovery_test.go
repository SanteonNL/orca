package smart_on_fhir

import (
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func TestDiscoverConfiguration(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		var requestURL string
		var acceptHeader string
		http.DefaultClient.Transport = roundTripFunc(func(req *http.Request) *http.Response {
			requestURL = req.URL.String()
			acceptHeader = req.Header.Get("Accept")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"token_endpoint":"http://example.com/token","authorization_endpoint":"http://example.com/authorize"}`)),
			}
		})

		fhirBaseURL, _ := url.Parse("http://example.com/fhir")
		config, err := DiscoverConfiguration(fhirBaseURL)
		require.NoError(t, err)
		// Assert HTTP request
		require.Equal(t, "http://example.com/fhir/.well-known/smart-configuration", requestURL)
		require.Equal(t, "application/json", acceptHeader)
		// Assert configuration
		require.Equal(t, "http://example.com/token", config.TokenEndpoint)
		require.Equal(t, "http://example.com/authorize", config.AuthorizationEndpoint)
	})
}
