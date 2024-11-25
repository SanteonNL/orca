package nuts

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"sync"
	"time"
)

var _ csd.Directory = &CsdDirectory{}

const cacheTtl = 30 * time.Second

func (d DutchNutsProfile) CsdDirectory() csd.Directory {
	return d.csd
}

// CsdDirectory is a CSD Directory that is backed by Nuts Discovery Services.
// It looks up fhir.Endpoint instances of owning entities (e.g. care organizations) in the Nuts Discovery Service.
type CsdDirectory struct {
	// APIClient is a REST API client to invoke the Nuts node's private Discovery Service API.
	APIClient  discovery.ClientWithResponsesInterface
	ServiceID  string
	entryCache map[string]cacheEntry
	cacheMux   sync.RWMutex
}

type cacheEntry struct {
	response discovery.SearchPresentationsResponse
	created  time.Time
}

// LookupEndpoint searches for endpoints of the given owner, with the given endpointName in the given Discovery Service.
// It queries the Nuts Discovery Service, translating the owner's identifier to a credential attribute (see IdentifierCredentialMapping).
// The endpoint is retrieved from the Nuts Discovery Service registration's registrationParameters, identified by endpointName.
func (n *CsdDirectory) LookupEndpoint(ctx context.Context, owner fhir.Identifier, endpointName string) ([]fhir.Endpoint, error) {
	response, err := n.find(ctx, owner)
	if err != nil {
		return nil, err
	}
	var results []fhir.Endpoint
	for _, searchResult := range *response.JSON200 {
		if searchResult.RegistrationParameters == nil {
			continue
		}
		endpoint, ok := searchResult.RegistrationParameters[endpointName].(string)
		if !ok {
			continue
		}
		results = append(results, fhir.Endpoint{
			Address: endpoint,
			Status:  fhir.EndpointStatusActive,
			ConnectionType: fhir.Coding{
				System: to.Ptr("http://hl7.org/fhir/ValueSet/endpoint-connection-type"),
				Code:   to.Ptr("hl7-fhir-rest"),
			},
		})
	}
	return results, nil
}

func (n *CsdDirectory) LookupEntity(ctx context.Context, identifier fhir.Identifier) (*fhir.Reference, error) {
	response, err := n.find(ctx, identifier)
	if err != nil {
		return nil, err
	}
	if len(*response.JSON200) == 0 {
		return nil, csd.ErrEntryNotFound
	}
	organizationName, hasName := (*response.JSON200)[0].Fields["organization_name"]
	result := fhir.Reference{
		Type:       to.Ptr("Organization"),
		Identifier: &identifier,
	}
	if hasName {
		result.Display = to.Ptr(organizationName.(string))
	}
	return &result, nil
}

func (n *CsdDirectory) find(ctx context.Context, owner fhir.Identifier) (*discovery.SearchPresentationsResponse, error) {
	if owner.Value == nil || owner.System == nil {
		return nil, errors.New("identifier must contain both System and Value")
	}
	if *owner.System != coolfhir.URANamingSystem {
		return nil, errors.New("identifier.system must be " + coolfhir.URANamingSystem)
	}

	// Check if the entry is cached
	cacheKey := fmt.Sprintf("%s|%s", *owner.System, *owner.Value)
	n.cacheMux.RLock()
	entry, isCached := n.entryCache[cacheKey]
	n.cacheMux.RUnlock()
	if isCached {
		if time.Since(entry.created) < cacheTtl {
			return &entry.response, nil
		}
		// evict
		n.cacheMux.Lock()
		delete(n.entryCache, cacheKey)
		n.cacheMux.Unlock()
	}

	// 2 credentials are supported:
	// - NutsUraCredential, which contains credentialSubject.organization.ura
	// - UziServerCertificateCredential, which contains credentialSubject.otherName (which is a string that contains the URA)
	//   Example otherName: 2.16.528.1.1007.99.2110-1-1234-S-86446-00.000-5678 (86446 is the URA)
	var searchResponse *discovery.SearchPresentationsResponse
	searchResponse, err := n.doSearch(ctx, discovery.SearchPresentationsParams{
		Query: &map[string]interface{}{
			"credentialSubject.otherName": "*-S-" + *owner.Value + "-00.000*",
		},
	})
	if err != nil {
		return nil, err
	}
	// Filter UziServerCertificateCredential to check actually match the URA, removing entries that don't match. Important since we do a wildcard match.
	j := 0
	for i := 0; i < len(*searchResponse.JSON200); i++ {
		if *owner.Value != (*searchResponse.JSON200)[i].Fields["organization_ura"] {
			continue
		}
		(*searchResponse.JSON200)[j] = (*searchResponse.JSON200)[i]
		j++
	}
	*searchResponse.JSON200 = (*searchResponse.JSON200)[:j]
	// If not found, try NutsUraCredential (backwards compatibility)
	if len(*searchResponse.JSON200) == 0 {
		searchResponse, err = n.doSearch(ctx, discovery.SearchPresentationsParams{
			Query: &map[string]interface{}{
				"credentialSubject.organization.ura": *owner.Value,
			},
		})
	}

	// Cache the entry
	n.cacheMux.Lock()
	n.entryCache[cacheKey] = cacheEntry{
		response: *searchResponse,
		created:  time.Now(),
	}
	n.cacheMux.Unlock()

	return searchResponse, nil
}

func (n *CsdDirectory) doSearch(ctx context.Context, query discovery.SearchPresentationsParams) (*discovery.SearchPresentationsResponse, error) {
	response, err := n.APIClient.SearchPresentationsWithResponse(ctx, n.ServiceID, &query)
	if err != nil {
		return nil, fmt.Errorf("search presentations: %w", err)
	}
	if response.JSON200 == nil || response.StatusCode() != http.StatusOK {
		if response.ApplicationproblemJSONDefault != nil {
			if response.ApplicationproblemJSONDefault.Status == http.StatusNotFound {
				return nil, errors.Join(csd.ErrEntryNotFound, fmt.Errorf("%s - %s", response.ApplicationproblemJSONDefault.Title, response.ApplicationproblemJSONDefault.Detail))
			}
			return nil, fmt.Errorf("search presentations non-OK HTTP response (status=%s): %v", response.Status(), response.ApplicationproblemJSONDefault)
		}
		return nil, fmt.Errorf("search presentations non-OK HTTP response (status=%s)", response.Status())
	}
	return response, nil
}
