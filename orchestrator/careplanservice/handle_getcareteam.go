package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

// handleGetCareTeam fetches the requested CareTeam and validates if the requester is a participant
// if the requester is valid, return the CareTeam, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetCareTeam(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.CareTeam, error) {
	// fetch CareTeam, validate requester is participant
	var careTeam fhir.CareTeam
	err := s.fhirClient.Read("CareTeam/"+id, &careTeam, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}
	err = validatePrincipalInCareTeams(ctx, []fhir.CareTeam{careTeam})
	if err != nil {
		return nil, err
	}

	return &careTeam, nil
}

// handleSearchCareTeam does a search for CareTeam based on the user requester parameters. Ensure only CareTeams for the requester's CarePlan are fetched
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCareTeam(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	params := []fhirclient.Option{}
	for k, v := range queryParams {
		for _, value := range v {
			params = append(params, fhirclient.QueryParam(k, value))
		}
	}
	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read("CareTeam", &bundle, params...)
	if err != nil {
		return nil, err
	}

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return nil, err
	}
	if len(careTeams) == 0 {
		// If there are no careTeams in the bundle there is no point in doing validation, return empty bundle to user
		return &bundle, nil
	}

	// For each CareTeam in bundle, validate the requester is a participant, and if not remove it from the bundle
	// This will be done by adding the IDs we do want to keep to a list, and then filtering the bundle based on this list
	IDs := make([]string, 0)
	for _, ct := range careTeams {
		err = validatePrincipalInCareTeams(ctx, []fhir.CareTeam{ct})
		if err != nil {
			continue
		}
		IDs = append(IDs, *ct.Id)
	}
	filterMissingResourcesOutOfBundle(&bundle, "CareTeam", IDs)

	return &bundle, nil
}
