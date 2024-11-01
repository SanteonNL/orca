package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
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
	carePlan, careTeams, headers, err := s.getCarePlanAndCareTeams("CarePlan/" + id)
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
		// Skip param to include CareTeam since we need to add this for validation anyway
		if k == "_include" && v[0] == "CarePlan:care-team" {
			continue
		}
		params = append(params, fhirclient.QueryParam(k, v[0]))
	}
	params = append(params, fhirclient.QueryParam("_include", "CarePlan:care-team"))
	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read("CarePlan", &bundle, params...)
	if err != nil {
		return nil, err
	}

	if len(bundle.Entry) == 0 {
		log.Info().Msg("CarePlan search returned empty bundle")
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

	err = validatePrincipalInCareTeams(ctx, careTeams)
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}
