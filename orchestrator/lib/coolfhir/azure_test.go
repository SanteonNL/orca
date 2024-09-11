package coolfhir

import (
	"encoding/json"
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
	var capturedHeaders http.Header
	mux.HandleFunc("/Patient/123", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedJSON)
		capturedReadQueryParams = r.URL.Query()
		capturedHeaders = r.Header
	})
	var capturedCreateBody []byte
	mux.HandleFunc("/Patient", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		capturedCreateBody, _ = io.ReadAll(r.Body)
		capturedHeaders = r.Header

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(expectedJSON)
	})
	testFHIRServer := httptest.NewTLSServer(mux)
	fhirBaseURL, _ := url.Parse(testFHIRServer.URL)
	http.DefaultClient = testFHIRServer.Client()

	// Create client
	fhirClient := NewAzureFHIRClient(fhirBaseURL, &fake.TokenCredential{})

	t.Run("Read resource", func(t *testing.T) {
		var actual fhir.Patient
		err := fhirClient.Read("Patient/123", &actual, fhirclient.QueryParam("foo", "bar"))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		require.Len(t, capturedReadQueryParams, 1)
		require.Equal(t, "bar", capturedReadQueryParams.Get("foo"))
		require.Equal(t, "Bearer fake_token", capturedHeaders.Get("Authorization"))
	})
	t.Run("Create resource", func(t *testing.T) {
		var actual fhir.Patient
		err := fhirClient.Create(expected, &actual)
		require.NoError(t, err)

		require.Equal(t, expected, actual)
		require.JSONEq(t, string(expectedJSON), string(capturedCreateBody))
		require.Equal(t, "application/fhir+json", capturedHeaders.Get("Content-Type"))
	})
}

func Test_NewFHIRAuthRoundTripper(t *testing.T) {
	t.Run("Invalid RoundTripper - Invalid FHIR URL", func(t *testing.T) {
		config := FHIRRoundTripperConfig{
			BaseURL: "ayiwrq284-02uwqa'trki::$juqwa58tp[9{{}{{}{}{Pwa",
			Auth:    FHIRAuthConfig{},
		}
		fhirClientConfig := Config()

		roundTripper, fhirClient, err := NewFHIRAuthRoundTripper(config, fhirClientConfig)
		require.Error(t, err)
		require.Nil(t, roundTripper)
		require.Nil(t, fhirClient)
	})
	t.Run("Invalid RoundTripper - Invalid Auth Type", func(t *testing.T) {
		config := FHIRRoundTripperConfig{
			BaseURL: "",
			Auth:    FHIRAuthConfig{Type: "foo"},
		}
		fhirClientConfig := Config()

		roundTripper, fhirClient, err := NewFHIRAuthRoundTripper(config, fhirClientConfig)
		require.EqualError(t, err, "invalid FHIR authentication type: foo")
		require.Nil(t, roundTripper)
		require.Nil(t, fhirClient)
	})
	t.Run("Valid RoundTripper - azuremanaged-identity", func(t *testing.T) {
		config := FHIRRoundTripperConfig{
			BaseURL: "",
			Auth: FHIRAuthConfig{
				Type: AzureManagedIdentity,
			},
		}
		fhirClientConfig := Config()

		roundTripper, fhirClient, err := NewFHIRAuthRoundTripper(config, fhirClientConfig)
		require.NoError(t, err)
		require.NotNil(t, roundTripper)
		require.NotNil(t, fhirClient)
		require.Equal(t, fhirClientConfig.MaxResponseSize, 10485760)
	})
	t.Run("Valid RoundTripper - default", func(t *testing.T) {
		config := FHIRRoundTripperConfig{
			BaseURL: "",
			Auth:    FHIRAuthConfig{},
		}
		fhirClientConfig := Config()

		roundTripper, fhirClient, err := NewFHIRAuthRoundTripper(config, fhirClientConfig)
		require.NoError(t, err)
		require.NotNil(t, roundTripper)
		require.Equal(t, roundTripper, http.DefaultTransport)
		require.NotNil(t, fhirClient)
		require.Equal(t, fhirClientConfig.MaxResponseSize, 10485760)
	})
}
