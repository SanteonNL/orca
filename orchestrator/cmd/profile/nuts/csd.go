package nuts

import (
	"context"
	"errors"
	"fmt"
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
	APIClient discovery.ClientWithResponsesInterface
	ServiceID string
	// IdentifierCredentialMapping maps logical identifiers to attributes in credentials in the Discovery Service's registrations.
	// For instance, to map the following FHIR identifier to a credential attribute:
	// {
	//   "system": "urn:simple:organization_name",
	//   "value": "123456789"
	// }
	// Given the following credential:
	// {
	//   "credentialSubject": {
	//     "organization": {
	//       "name": "Acme Hospital",
	//       "id": "123456789"
	//     }
	//   }
	// }
	// The value of the mapping would then be "credentialSubject.organization.name".
	IdentifierCredentialMapping map[string]string

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
	identifierSearchParam, supported := n.IdentifierCredentialMapping[*owner.System]
	if !supported {
		return nil, fmt.Errorf("no FHIR->Nuts Discovery Service mapping for CodingSystem: %s", *owner.System)
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

	response, err := n.APIClient.SearchPresentationsWithResponse(ctx, n.ServiceID, &discovery.SearchPresentationsParams{
		Query: &map[string]interface{}{
			identifierSearchParam: *owner.Value,
		},
	})
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

	// Cache the entry
	n.cacheMux.Lock()
	n.entryCache[cacheKey] = cacheEntry{
		response: *response,
		created:  time.Now(),
	}
	n.cacheMux.Unlock()

	return response, nil
}
