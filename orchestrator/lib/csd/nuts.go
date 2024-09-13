package csd

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/nuts-foundation/go-nuts-client/nuts/discovery"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

var _ Directory = &NutsDirectory{}

// NutsDirectory is a CSD Directory that is backed by Nuts Discovery Services.
type NutsDirectory struct {
	// APIClient is a REST API client to invoke the Nuts node's private Discovery Service API.
	APIClient discovery.ClientWithResponsesInterface
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

func (n NutsDirectory) LookupEndpoint(ctx context.Context, owner fhir.Identifier, service string, endpointName string) ([]fhir.Endpoint, error) {
	if !coolfhir.IsLogicalIdentifier(&owner) {
		return nil, errors.New("owner must be a logical identifier")
	}
	identifierSearchParam, supported := n.IdentifierCredentialMapping[*owner.System]
	if !supported {
		return nil, fmt.Errorf("no FHIR->Nuts Discovery Service mapping for system: %s", *owner.System)
	}
	response, err := n.APIClient.SearchPresentationsWithResponse(ctx, service, &discovery.SearchPresentationsParams{
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
				return nil, errors.Join(ErrEntryNotFound, fmt.Errorf("%s - %s", response.ApplicationproblemJSONDefault.Title, response.ApplicationproblemJSONDefault.Detail))
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
		// TODO: Supporting non-FHIR endpoints (e.g. Nuts Authorization Server URL) strictly requires a FHIR profile to define the values.
		results = append(results, fhir.Endpoint{
			Address: endpoint,
			Status:  fhir.EndpointStatusActive,
		})
	}
	return results, nil
}
