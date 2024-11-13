package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

// handleGetCarePlan fetches the requested CarePlan and validates if the requester has access to the resource (is a participant of one of the CareTeams of the care plan)
// if the requester is valid, return the CarePlan, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetCarePlan(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.CarePlan, error) {
	// fetch CarePlan + CareTeam, validate requester is participant of CareTeam
	// headers are passed in by reference and returned to the calling method
	carePlan, careTeams, headers, err := s.getCarePlanAndCareTeams(id)
	if err != nil {
		return nil, err
	}

	err = validatePrincipalInCareTeams(ctx, careTeams)
	if err != nil {
		return nil, err
	}

	return &carePlan, nil
}

// handleSearchCarePlan does a search for CarePlan based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCarePlan(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	params := []fhirclient.Option{}
	for k, v := range queryParams {
		for _, value := range v {
			// Skip param to include CareTeam since we need to add this for validation anyway
			if k == "_include" && value == "CarePlan:care-team" {
				continue
			}
			params = append(params, fhirclient.QueryParam(k, value))
		}
	}
	params = append(params, fhirclient.QueryParam("_include", "CarePlan:care-team"))
	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read("CarePlan", &bundle, params...)
	if err != nil {
		return nil, err
	}

	if len(bundle.Entry) == 0 {
		return &bundle, nil
	}

	var carePlans []fhir.CarePlan
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CarePlan"), &carePlans)
	if err != nil {
		return nil, err
	}
	if len(carePlans) == 0 {
		// If there are no carePlans in the bundle there is no point in doing validation, return empty bundle to user
		return &bundle, nil
	}

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return nil, err
	}
	if len(careTeams) == 0 {
		return nil, coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
	}

	// For each CareTeam in bundle, validate the requester is a participant, and if not remove it from the bundle
	// This will be done by adding the IDs we do want to keep to a list, and then filtering the bundle based on this list
	//careTeamIDs := make([]string, 0)
	carePlanIDs := make([]string, 0)
	for _, ct := range careTeams {
		err = validatePrincipalInCareTeams(ctx, []fhir.CareTeam{ct})
		if err != nil {
			continue
		}
		for _, cp := range carePlans {
			for _, cpct := range cp.CareTeam {
				if *cpct.Reference == fmt.Sprintf("CareTeam/%s", *ct.Id) {
					carePlanIDs = append(carePlanIDs, *cp.Id)
				}
			}
		}
	}
	retBundle := filterMatchingResourcesInBundle(&bundle, "CarePlan", carePlanIDs)

	return &retBundle, nil
}
