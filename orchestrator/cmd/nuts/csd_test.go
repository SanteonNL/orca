package nuts

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNutsDirectory_LookupEndpoint(t *testing.T) {
	ownerUnsupportedCodingSystem := fhir.Identifier{
		System: to.Ptr("custom"),
		Value:  to.Ptr("123456789"),
	}
	ownerURACodingSystem := fhir.Identifier{
		System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
		Value:  to.Ptr("123"),
	}
	const serviceID = "svc-test"
	const urlEndpointID = "url-endpoint"
	const mapEndpointID = "map-endpoint"
	const endpoint = "https://example.com/fhir"
	discoveryServerRouter := http.NewServeMux()
	discoveryServerRouter.HandleFunc("/internal/discovery/v1/"+serviceID, func(w http.ResponseWriter, r *http.Request) {
		var response []discovery.SearchResult
		if r.URL.Query().Get("credentialSubject.organization.ura") == *ownerURACodingSystem.Value {
			response = []discovery.SearchResult{
				{
					RegistrationParameters: map[string]interface{}{
						urlEndpointID: endpoint,
						mapEndpointID: map[string]interface{}{
							"address": endpoint,
						},
					},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})
	discoveryServer := httptest.NewServer(discoveryServerRouter)
	apiClient, _ := discovery.NewClientWithResponses(discoveryServer.URL)
	directory := NutsDirectory{
		IdentifierCredentialMapping: map[string]string{
			"http://fhir.nl/fhir/NamingSystem/ura": "credentialSubject.organization.ura", // URACredential
		},
		APIClient: apiClient,
	}
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, serviceID, urlEndpointID)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Equal(t, endpoint, result[0].Address)
		require.Equal(t, fhir.EndpointStatusActive, result[0].Status)
	})
	t.Run("FHIR CodingSystem not mapped to Verifiable Credential property", func(t *testing.T) {
		_, err := directory.LookupEndpoint(ctx, ownerUnsupportedCodingSystem, serviceID, urlEndpointID)
		require.EqualError(t, err, "no FHIR->Nuts Discovery Service mapping for CodingSystem: custom")
	})
	t.Run("non-OK status", func(t *testing.T) {
		result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, serviceID, mapEndpointID)
		require.NoError(t, err)
		require.Empty(t, result)
	})
	t.Run("endpoint is not present", func(t *testing.T) {
		result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, "unknown", urlEndpointID)
		require.EqualError(t, err, "search presentations non-OK HTTP response (status=404 Not Found)")
		require.Empty(t, result)
	})
	t.Run("endpoint is not a string", func(t *testing.T) {
		result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, serviceID, mapEndpointID)
		require.NoError(t, err)
		require.Empty(t, result)
	})
}
