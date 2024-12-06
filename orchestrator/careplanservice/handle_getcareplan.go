package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"slices"
)

// handleGetCarePlan fetches the requested CarePlan and validates if the requester has access to the resource (is a participant of one of the CareTeams of the care plan)
// if the requester is valid, return the CarePlan, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetCarePlan(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.CarePlan, error) {
	// fetch CarePlan + CareTeam, validate requester is participant of CareTeam
	// headers are passed in by reference and returned to the calling method
	carePlan, careTeams, headers, err := s.getCarePlanAndCareTeams(ctx, "CarePlan/"+id)
	if err != nil {
		return nil, err
	}

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = validatePrincipalInCareTeams(principal, careTeams)
	if err != nil {
		return nil, err
	}

	return &carePlan, nil
}

// handleSearchCarePlan does a search for CarePlan based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCarePlan(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	// Verify requester is authenticated
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// Check if CareTeam is included in the response, if not add it to the query
	includeCareTeamInResponse := false
	if slices.Contains(queryParams["_include"], "CarePlan:care-team") {
		includeCareTeamInResponse = true
	} else {
		queryParams.Add("_include", "CarePlan:care-team")
	}

	carePlans, bundle, err := handleSearchResource[fhir.CarePlan](s, ctx, "CarePlan", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(carePlans) == 0 {
		// If there are no carePlans in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return nil, err
	}
	if len(careTeams) == 0 {
		return nil, coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
	}

	// For each CareTeam in bundle, validate the requester is a participant, and if not remove it from the bundle
	// This will be done by adding the IDs we do want to keep to a list, and then filtering the bundle based on this list
	filterRefs := make([]string, 0)
	for _, ct := range careTeams {
		err = validatePrincipalInCareTeams(principal, []fhir.CareTeam{ct})
		if err != nil {
			continue
		}
		if includeCareTeamInResponse {
			filterRefs = append(filterRefs, "CareTeam/"+*ct.Id)
		}
		for _, cp := range carePlans {
			for _, cpct := range cp.CareTeam {
				if *cpct.Reference == fmt.Sprintf("CareTeam/%s", *ct.Id) {
					filterRefs = append(filterRefs, "CarePlan/"+*cp.Id)
				}
			}
		}
	}
	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"CarePlan", "CareTeam"}, filterRefs)

	return &retBundle, nil
}
