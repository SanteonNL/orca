package main

import (
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
	fhirMux.Handle("GET /Task/1", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccessToken = r.Header.Get("Authorization")
		capturedQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType":"Task","id":"1"}`))
	}))
	defer fhirServer.Close()
	fhirBaseURL, _ := url.Parse(fhirServer.URL)

	reverseProxy, err := create("test/private.jwk", "7c87bdbc-ef89-41a7-a940-70d7e7b1a828", fhirBaseURL, "test")
	require.NoError(t, err)

	proxyServer := httptest.NewServer(reverseProxy)
	defer proxyServer.Close()

	// Call front endpoint of the proxy (read Task resource), which should call the FHIR server with an access token
	httpResponse, err := http.Get(proxyServer.URL + "/Task/1?foo=bar&msg=Hello,+World!")
	require.NoError(t, err)

	// Assert response return by FHIR mux
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, "Bearer access_token", capturedAccessToken)
	data, err := io.ReadAll(httpResponse.Body)
	require.NoError(t, err)
	require.Equal(t, `{"resourceType":"Task","id":"1"}`, string(data))
	assert.Equal(t, url.Values{"foo": []string{"bar"}, "msg": []string{"Hello, World!"}}, capturedQuery)
}
