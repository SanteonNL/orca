package nuts

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

var _ csd.Directory = &CsdDirectory{}

func (d DutchNutsProfile) CsdDirectory() csd.Directory {
	apiClient, _ := discovery.NewClientWithResponses(d.Config.API.URL)
	return CsdDirectory{
		APIClient: apiClient,
		IdentifierCredentialMapping: map[string]string{
			coolfhir.URANamingSystem: "credentialSubject.organization.ura", // NutsURACredential provides URA attribute
		},
	}
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
}

// LookupEndpoint searches for endpoints of the given owner, with the given endpointName in the given Discovery Service.
// It queries the Nuts Discovery Service, translating the owner's identifier to a credential attribute (see IdentifierCredentialMapping).
// The endpoint is retrieved from the Nuts Discovery Service registration's registrationParameters, identified by endpointName.
func (n CsdDirectory) LookupEndpoint(ctx context.Context, owner fhir.Identifier, endpointName string) ([]fhir.Endpoint, error) {
	identifierSearchParam, supported := n.IdentifierCredentialMapping[*owner.System]
	if !supported {
		return nil, fmt.Errorf("no FHIR->Nuts Discovery Service mapping for CodingSystem: %s", *owner.System)
	}
	response, err := n.APIClient.SearchPresentationsWithResponse(ctx, n.ServiceID, &discovery.SearchPresentationsParams{
		Query: &map[string]interface{}{
			identifierSearchParam: *owner.Value,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search presentations: %w", err)
	}
	if response.JSON200 == nil {
		if response.ApplicationproblemJSONDefault != nil {
			if response.ApplicationproblemJSONDefault.Status == http.StatusNotFound {
				return nil, errors.Join(csd.ErrEntryNotFound, fmt.Errorf("%s - %s", response.ApplicationproblemJSONDefault.Title, response.ApplicationproblemJSONDefault.Detail))
			}
			return nil, fmt.Errorf("search presentations non-OK HTTP response (status=%s): %v", response.Status(), response.ApplicationproblemJSONDefault)
		}
		return nil, fmt.Errorf("search presentations non-OK HTTP response (status=%s)", response.Status())
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
