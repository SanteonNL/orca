package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleSearchResource is a generic function to search for a resource of a given type and return the results
// it returns a processed list of the required resource type, the full bundle and an error
func handleSearchResource[T any](ctx context.Context, fhirClient fhirclient.Client, resourceType string, queryParams url.Values, headers *fhirclient.Headers) ([]T, *fhir.Bundle, error) {
	form := url.Values{}
	for k, v := range queryParams {
		form.Add(k, strings.Join(v, ","))
	}
	var bundle fhir.Bundle
	err := fhirClient.SearchWithContext(ctx, resourceType, form, &bundle, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	var resources []T
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType(resourceType), &resources)
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	return resources, &bundle, nil
}

func validatePrincipalInCareTeam(principal auth.Principal, careTeam *fhir.CareTeam) error {
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeam, principal.Organization.Identifier)
	if participant == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Participant is not part of CareTeam",
			StatusCode: http.StatusForbidden,
		}
	}
	return nil
}

func updateMetaSource(resource any, fhirBaseURL *url.URL) {
	resourceType := coolfhir.ResourceType(resource)
	resourceID := coolfhir.ResourceID(resource)
	if resourceID == nil {
		return
	}
	source := fhirBaseURL.JoinPath(resourceType, *resourceID).String()
	coolfhir.SetSource(resource, to.Ptr(source))
}
