package nuts

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCsdDirectory_Lookup(t *testing.T) {
	ownerUnsupportedCodingSystem := &fhir.Identifier{
		System: to.Ptr("custom"),
		Value:  to.Ptr("123456789"),
	}
	ownerURACodingSystem := &fhir.Identifier{
		System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
		Value:  to.Ptr("123"),
	}
	otherOwnerURACodingSystem := &fhir.Identifier{
		System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
		Value:  to.Ptr("456"),
	}
	const serviceID = "svc-test"
	const urlEndpointID = "url-endpoint"
	const mapEndpointID = "map-endpoint"
	const endpoint = "https://example.com/fhir"
	const otherEndpoint = "https://example.com/other/fhir"
	numInvocations := new(atomic.Int32)
	discoveryServerRouter := http.NewServeMux()
	discoveryServerRouter.HandleFunc("/internal/discovery/v1/"+serviceID, func(w http.ResponseWriter, r *http.Request) {
		numInvocations.Add(1)
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
					Fields: map[string]interface{}{
						"organization_name": "example",
					},
				},
			}
		}
		if r.URL.Query().Get("credentialSubject.otherName") == "*-S-"+*otherOwnerURACodingSystem.Value+"-00.000*" {
			response = []discovery.SearchResult{
				{
					RegistrationParameters: map[string]interface{}{
						urlEndpointID: otherEndpoint,
						mapEndpointID: map[string]interface{}{
							"address": otherEndpoint,
						},
					},
					Fields: map[string]interface{}{
						"organization_ura":  "456",
						"organization_name": "other",
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
	directory := CsdDirectory{
		APIClient:  apiClient,
		ServiceID:  serviceID,
		entryCache: make(map[string]cacheEntry),
		cacheMux:   sync.RWMutex{},
	}
	ctx := context.Background()
	t.Run("LookupEndpoint", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, urlEndpointID)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, endpoint, result[0].Address)
			require.Equal(t, fhir.EndpointStatusActive, result[0].Status)
		})
		t.Run("UziServerCertificateCredential", func(t *testing.T) {
			result, err := directory.LookupEndpoint(ctx, otherOwnerURACodingSystem, urlEndpointID)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, otherEndpoint, result[0].Address)
			require.Equal(t, fhir.EndpointStatusActive, result[0].Status)
		})
		t.Run("cached", func(t *testing.T) {
			numInvocations.Store(0)
			directory.entryCache = map[string]cacheEntry{}

			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, urlEndpointID)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, 2, int(numInvocations.Load()))
			_, _ = directory.LookupEndpoint(ctx, ownerURACodingSystem, urlEndpointID)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, 2, int(numInvocations.Load()))
		})
		t.Run("stale cache", func(t *testing.T) {
			numInvocations.Store(0)
			directory.entryCache = map[string]cacheEntry{}

			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, urlEndpointID)
			require.NoError(t, err)
			require.Len(t, result, 1)
			require.Equal(t, 2, int(numInvocations.Load()))
			directory.cacheMux.Lock()
			directory.entryCache[*ownerURACodingSystem.System+"|"+*ownerURACodingSystem.Value] = cacheEntry{
				created: time.Time{},
			}
			directory.cacheMux.Unlock()
			_, _ = directory.LookupEndpoint(ctx, ownerURACodingSystem, urlEndpointID)
			require.Equal(t, 4, int(numInvocations.Load()))
		})
		t.Run("FHIR CodingSystem not mapped to Verifiable Credential property", func(t *testing.T) {
			_, err := directory.LookupEndpoint(ctx, ownerUnsupportedCodingSystem, urlEndpointID)
			require.EqualError(t, err, "identifier.system must be http://fhir.nl/fhir/NamingSystem/ura")
		})
		t.Run("non-OK status", func(t *testing.T) {
			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, mapEndpointID)
			require.NoError(t, err)
			require.Empty(t, result)
		})
		t.Run("unknown service", func(t *testing.T) {
			directory := CsdDirectory{
				APIClient: apiClient,
				ServiceID: "unknown",
			}
			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, endpoint)
			require.EqualError(t, err, "search presentations non-OK HTTP response (status=404 Not Found)")
			require.Empty(t, result)
		})
		t.Run("endpoint is not present", func(t *testing.T) {
			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, "unknown")
			require.NoError(t, err)
			require.Empty(t, result)
		})
		t.Run("endpoint is not a string", func(t *testing.T) {
			result, err := directory.LookupEndpoint(ctx, ownerURACodingSystem, mapEndpointID)
			require.NoError(t, err)
			require.Empty(t, result)
		})
	})
	t.Run("LookupEntity", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			result, err := directory.LookupEntity(ctx, *ownerURACodingSystem)
			require.NoError(t, err)
			require.Equal(t, "Organization", *result.Type)
			require.Equal(t, *ownerURACodingSystem, *result.Identifier)
			require.NotNil(t, result.Display)
			require.Equal(t, "example", *result.Display)
		})
		t.Run("no results", func(t *testing.T) {
			result, err := directory.LookupEntity(ctx, fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("unknown"),
			})
			require.ErrorIs(t, err, csd.ErrEntryNotFound)
			require.Nil(t, result)
		})
	})
}
