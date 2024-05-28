package main

import (
	"encoding/json"
	"github.com/SanteonNL/orca/smartonfhir_backend_adapter/keys"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func Test_create(t *testing.T) {
	fhirMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirMux)
	fhirMux.Handle("GET /.well-known/smart-configuration", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token_endpoint":"` + fhirServer.URL + `/token","authorization_endpoint":"` + fhirServer.URL + `/authorize"}`))
	}))
	fhirMux.Handle("POST /token", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"access_token","token_type":"bearer","expires_in":3600}`))
	}))
	var capturedAccessToken string
	var capturedQuery url.Values
	var capturedHeaders http.Header
	var capturedHost string
	fhirMux.Handle("GET /Task/1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccessToken = r.Header.Get("Authorization")
		capturedQuery = r.URL.Query()
		capturedHeaders = r.Header.Clone()
		capturedHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Task","id":"1"}`))
	}))
	fhirMux.Handle("GET /Task/error", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer fhirServer.Close()
	fhirBaseURL, _ := url.Parse(fhirServer.URL)

	signingKey, err := keys.SigningKeyFromJWKFile("test/private.jwk")
	require.NoError(t, err)

	reverseProxy, err := create(signingKey, fhirBaseURL, "test")
	require.NoError(t, err)

	proxyServer := httptest.NewServer(reverseProxy)
	defer proxyServer.Close()

	t.Run("ok", func(t *testing.T) {
		// Call front endpoint of the proxy (read Task resource), which should call the FHIR server with an access token
		httpRequest, _ := http.NewRequest("GET", proxyServer.URL+"/Task/1?foo=bar&msg=Hello,+World!", nil)
		httpRequest.Header.Set("WWW-Authenticate", "Bearer access_token")
		httpRequest.Header.Set("Content-Type", "application/fhir+json")
		httpRequest.Header.Set("Accept", "application/fhir+json")
		httpResponse, err := http.DefaultClient.Do(httpRequest)
		require.NoError(t, err)

		// Assert response return by FHIR mux
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, "Bearer access_token", capturedAccessToken)
		data, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err)
		// Assert request body is retained
		require.Equal(t, `{"resourceType":"Task","id":"1"}`, string(data))
		// Assert query parameters are retained
		assert.Equal(t, url.Values{"foo": []string{"bar"}, "msg": []string{"Hello, World!"}}, capturedQuery)
		// Assert host is not retained
		assert.Equal(t, fhirBaseURL.Host, capturedHost)
		// Assert headers are cleaned, some are retained
		assert.Empty(t, capturedHeaders.Get("WWW-Authenticate"))
		assert.Equal(t, "application/fhir+json", capturedHeaders.Get("Accept"))
		assert.Equal(t, "application/fhir+json", capturedHeaders.Get("Content-Type"))
	})
	t.Run("FHIR OperationOutcome on error", func(t *testing.T) {
		httpResponse, err := http.Get(proxyServer.URL + "/Task/error")
		require.NoError(t, err)
		responseData, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err)
		var operationOutcome fhir.OperationOutcome
		require.NoError(t, json.Unmarshal(responseData, &operationOutcome))
		// Assert OperationOutcome
		assert.Equal(t, fhir.IssueSeverityError, operationOutcome.Issue[0].Severity)
		assert.Equal(t, "The system tried to proxy the FHIR operation, but an error occurred.", *operationOutcome.Issue[0].Diagnostics)
	})
}
