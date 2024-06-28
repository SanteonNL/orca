package careplanservice

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestService_Proxy(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	service, err := New(Config{
		FHIR: FHIRConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, nil)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpResponse, err := frontServer.Client().Get(frontServer.URL + "/cps/Patient")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
}

func TestNew(t *testing.T) {
	t.Run("FHIR server URL not configured", func(t *testing.T) {
		_, err := New(Config{}, nil)
		require.EqualError(t, err, "careplancontributor.careplanservice.url is not configured")
	})
	t.Run("unknown FHIR server auth type", func(t *testing.T) {
		_, err := New(Config{
			FHIR: FHIRConfig{
				BaseURL: "http://example.com",
				Auth:    FHIRAuthConfig{Type: "foo"},
			},
		}, nil)
		require.EqualError(t, err, "invalid FHIR authentication type: foo")
	})
}
