package coolfhir

import (
	"encoding/json"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func Test_azureHttpClient_Do(t *testing.T) {
	expected := fhir.Patient{
		Id: to.Ptr("123"),
	}
	expectedJSON, _ := json.Marshal(expected)

	mux := http.NewServeMux()
	var capturedReadQueryParams url.Values
	mux.HandleFunc("/Patient/123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedJSON)
		capturedReadQueryParams = r.URL.Query()
	})
	var capturedCreateBody []byte
	var capturedHeaders http.Header
	mux.HandleFunc("/Patient", func(w http.ResponseWriter, r *http.Request) {
		capturedCreateBody, _ = io.ReadAll(r.Body)
		capturedHeaders = r.Header

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedJSON)
	})
	testFHIRServer := httptest.NewTLSServer(mux)
	fhirBaseURL, _ := url.Parse(testFHIRServer.URL)

	// Create client
	fhirClient, err := newAzureClientWithCredential(fhirBaseURL, &fake.TokenCredential{}, azcore.ClientOptions{
		Transport: testFHIRServer.Client(),
	})
	require.NoError(t, err)

	t.Run("Read resource", func(t *testing.T) {
		var actual fhir.Patient
		err = fhirClient.Read("Patient/123", &actual, fhirclient.QueryParam("foo", "bar"))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Len(t, capturedReadQueryParams, 1)
		require.Equal(t, "bar", capturedReadQueryParams.Get("foo"))
	})
	t.Run("Create resource", func(t *testing.T) {
		var actual fhir.Patient
		err = fhirClient.Create(expected, &actual)
		require.NoError(t, err)

		require.Equal(t, expected, actual)
		require.JSONEq(t, string(expectedJSON), string(capturedCreateBody))
		require.Equal(t, "application/fhir+json", capturedHeaders.Get("Content-Type"))
	})
}
