package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")
var nutsPublicURL, _ = url.Parse("https://example.com/nuts")

func TestService_Proxy(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	// Setup: create the service
	service, err := New(Config{
		FHIR: FHIRConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, nutsPublicURL, orcaPublicURL, "", nil)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport)

	httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Patient")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)
}

func Test_HandleProtectedResourceMetadata(t *testing.T) {
	// Test that the service handles the protected resource metadata URL
	// Setup: configure the service
	service, err := New(Config{
		FHIR: FHIRConfig{
			BaseURL: "http://example.com",
		},
	}, nutsPublicURL, orcaPublicURL, "did:web:example.com", nil)
	require.NoError(t, err)
	// Setup: configure the service to handle the protected resource metadata URL
	serverMux := http.NewServeMux()
	service.RegisterHandlers(serverMux)
	server := httptest.NewServer(serverMux)

	httpResponse, err := server.Client().Get(server.URL + "/cps/.well-known/oauth-protected-resource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

}

func TestNew(t *testing.T) {
	t.Run("unknown FHIR server auth type", func(t *testing.T) {
		_, err := New(Config{
			FHIR: FHIRConfig{
				BaseURL: "http://example.com",
				Auth:    FHIRAuthConfig{Type: "foo"},
			},
		}, nutsPublicURL, orcaPublicURL, "", nil)
		require.EqualError(t, err, "invalid FHIR authentication type: foo")
	})
}
