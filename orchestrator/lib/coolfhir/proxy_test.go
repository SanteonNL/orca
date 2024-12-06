package coolfhir

import (
	"bytes"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
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
	upstreamServerMux.HandleFunc("GET /fhir/DoGet", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		capturedCookies = request.Cookies()
		capturedHeaders = request.Header
		writer.Write([]byte(`{"resourceType":"Patient"}`))
	})
	upstreamServerMux.HandleFunc("POST /fhir/DoPost", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		capturedCookies = request.Cookies()
		capturedHeaders = request.Header
		writer.Write([]byte(`{"resourceType":"Patient"}`))
	})
	upstreamServerMux.HandleFunc("PUT /fhir/DoPut", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		capturedCookies = request.Cookies()
		capturedHeaders = request.Header
		writer.Write([]byte(`{"resourceType":"Patient"}`))
	})
	upstreamServerMux.HandleFunc("GET /fhir/InvalidJsonResponse", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		capturedCookies = request.Cookies()
		capturedHeaders = request.Header
		writer.Write([]byte(`this is not JSON`))
	})
	upstreamServer := httptest.NewServer(upstreamServerMux)
	upstreamServerURL, _ := url.Parse(upstreamServer.URL)
	upstreamServerURL.Path = "/fhir"

	// Configure proxy server
	proxyServerMux := http.NewServeMux()
	proxyTransportRequestHeaders := make(http.Header)
	proxyBaseUrl, _ := url.Parse("http://localhost/localfhir")
	proxy := NewProxy("Test", log.Logger, upstreamServerURL, "/localfhir", proxyBaseUrl, decoratingRoundTripper{
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

	t.Run("fhirclient", func(t *testing.T) {
		baseUrl, _ := url.Parse(proxyServer.URL + "/localfhir")
		client := fhirclient.New(baseUrl, http.DefaultClient, nil)
		var patient fhir.Patient
		err := client.Read("DoGet", &patient, fhirclient.QueryParam("_id", "1"))
		require.NoError(t, err)
		require.Equal(t, "1", capturedQueryParams.Get("_id"))
	})
	t.Run("upstream error", func(t *testing.T) {
		t.Run("invalid FHIR response (not valid JSON)", func(t *testing.T) {
			httpResponse, err := proxyServer.Client().Get(proxyServer.URL + "/localfhir/InvalidJsonResponse")
			require.NoError(t, err)
			require.Equal(t, http.StatusBadGateway, httpResponse.StatusCode)
			responseData, _ := io.ReadAll(httpResponse.Body)
			assert.Contains(t, string(responseData), "FHIR response unmarshal failed")
		})
	})
	t.Run("GET request", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Empty(t, capturedQueryParams)
		assert.Empty(t, capturedCookies)
	})
	t.Run("POST request", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("POST", proxyServer.URL+"/localfhir/DoPost", bytes.NewReader([]byte(`{"resourceType":"Patient"}`)))
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, httpResponse.StatusCode)
			assert.Empty(t, capturedQueryParams)
			assert.Empty(t, capturedCookies)
		})
		t.Run("missing request body", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("POST", proxyServer.URL+"/localfhir/DoPost", nil)
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			responseData, _ := io.ReadAll(httpResponse.Body)
			assert.Contains(t, string(responseData), "Request body is required for POST requests")
		})
		t.Run("invalid JSON in request body", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("POST", proxyServer.URL+"/localfhir/DoPost", bytes.NewReader([]byte(`{invalid json}`)))
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			responseData, _ := io.ReadAll(httpResponse.Body)
			assert.Contains(t, string(responseData), "Request body isn't valid JSON")
		})
	})
	t.Run("PUT request", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("PUT", proxyServer.URL+"/localfhir/DoPut", bytes.NewReader([]byte(`{"resourceType":"Patient"}`)))
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, httpResponse.StatusCode)
			assert.Empty(t, capturedQueryParams)
			assert.Empty(t, capturedCookies)
		})
		t.Run("missing request body", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("PUT", proxyServer.URL+"/localfhir/DoPut", nil)
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			assert.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			responseData, _ := io.ReadAll(httpResponse.Body)
			assert.Contains(t, string(responseData), "Request body is required for PUT requests")
		})
	})
	t.Run("Host header is rewritten to upstream server hostname", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, upstreamServerURL.Host, capturedHost)
	})
	t.Run("query parameters are retained", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet?_search=foo:bar", nil)
		httpResponse, err := proxyServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "foo:bar", capturedQueryParams.Get("_search"))
		assert.Empty(t, capturedCookies)
	})
	t.Run("headers", func(t *testing.T) {
		t.Run("request headers are proxied", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
			httpRequest.Header.Set("Accept", "application/fhir+json")
			httpRequest.Header.Set("CustomHeader", "should be there")
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			assert.Equal(t, "application/fhir+json", capturedHeaders.Get("Accept"))
			assert.Equal(t, "should be there", capturedHeaders.Get("CustomHeader"))
		})
		t.Run("cookies are not retained", func(t *testing.T) {
			// Cookies could contain sensitive information (session tokens), so they should not be proxied
			httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
			httpRequest.AddCookie(&http.Cookie{
				Name:  "sid",
				Value: "test",
			})
			httpResponse, err := proxyServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			assert.Empty(t, capturedCookies)
		})
		t.Run("request headers are sanitized", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
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
			httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/localfhir/DoGet", nil)
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
