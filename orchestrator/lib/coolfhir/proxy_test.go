package coolfhir

import (
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProxy(t *testing.T) {
	// Configure server being proxied to
	upstreamServerMux := http.NewServeMux()
	capturedHost := ""
	var capturedQueryParams url.Values
	var capturedCookies []*http.Cookie
	var capturedHeaders http.Header
	upstreamServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		capturedCookies = request.Cookies()
		capturedHeaders = request.Header
	})
	upstreamServer := httptest.NewServer(upstreamServerMux)
	upstreamServerURL, _ := url.Parse(upstreamServer.URL)
	upstreamServerURL.Path = "/fhir"

	// Configure proxy server
	proxyServerMux := http.NewServeMux()
	proxyTransportRequestHeaders := make(http.Header)
	proxy := NewProxy(log.Logger, upstreamServerURL, "/localfhir", decoratingRoundTripper{
		inner: http.DefaultTransport,
		decorator: func(request *http.Request) *http.Request {
			for name, value := range proxyTransportRequestHeaders {
				request.Header[name] = value
			}
			return request
		},
	})
	proxyServer := httptest.NewServer(proxyServerMux)
	proxyServerMux.HandleFunc("/localfhir/{rest...}", proxy.ServeHTTP)

	t.Run("base request", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Empty(t, capturedQueryParams)
		assert.Empty(t, capturedCookies)
	})
	t.Run("Host header is rewritten to upstream server hostname", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, upstreamServerURL.Host, capturedHost)
	})
	t.Run("query parameters are retained", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient?_search=foo:bar", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "foo:bar", capturedQueryParams.Get("_search"))
		assert.Empty(t, capturedCookies)
	})
	t.Run("cookies are not retained", func(t *testing.T) {
		// Cookies could contain sensitive information (session tokens), so they should not be proxied
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		httpRequest.AddCookie(&http.Cookie{
			Name:  "sid",
			Value: "test",
		})
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Empty(t, capturedCookies)
	})
	t.Run("request headers are proxied", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		httpRequest.Header.Set("Accept", "application/fhir+json")
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "application/fhir+json", capturedHeaders.Get("Accept"))
	})
	t.Run("request headers are sanitized", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		// There can be numerous X-headers that can contain information that should not be proxied by default (e.g. internal infrastructure details)
		httpRequest.Header.Set("X-Request-Id", "custom-client")
		// User agent can convey privacy-sensitive information about the client that should not be proxied
		httpRequest.Header.Set("User-Agent", "test")
		// Referer can contain sensitive information about the client's browsing history that should not be proxied
		httpRequest.Header.Set("Referer", "test")
		// Authorization header can contain sensitive information that should not be proxied.
		httpRequest.Header.Set("Authorization", "Bearer test")

		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.NotEqual(t, "custom-client", capturedHeaders.Get("User-Agent"))
		assert.Empty(t, capturedHeaders.Get("X-Request-Id"))
		assert.Empty(t, capturedHeaders.Get("Referer"))
		assert.Empty(t, capturedHeaders.Get("Authorization"))
	})
	t.Run("authorization header from passed http.Client is proxied", func(t *testing.T) {
		// Authorization header from the proxied request should not be proxied,
		// but if the http.Client used for proxying sets it, the latter one should be proxied
		// (used for authentication).
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/Patient", nil)
		httpRequest.Header.Set("Authorization", "Bearer test")
		defer func() {
			proxyTransportRequestHeaders = make(http.Header)
		}()
		proxyTransportRequestHeaders.Set("Authorization", "Bearer set-by-proxy-client")
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "Bearer set-by-proxy-client", capturedHeaders.Get("Authorization"))
	})
}

var _ http.RoundTripper = &decoratingRoundTripper{}

type decoratingRoundTripper struct {
	inner     http.RoundTripper
	decorator func(request *http.Request) *http.Request
}

func (d decoratingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return d.inner.RoundTrip(d.decorator(request))
}
