package careplanservice

import (
	"context"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetCarePlan fetches the requested CarePlan and validates if the requester has access to the resource (is a participant of one of the CareTeams of the care plan)
// if the requester is valid, return the CarePlan, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetCarePlan(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.CarePlan, error) {
	var carePlan fhir.CarePlan

	// fetch CarePlan, validate requester is participant of CareTeam
	// headers are passed in by reference and returned to the calling method
	err := s.fhirClient.ReadWithContext(ctx, "CarePlan/"+id, &carePlan)
	if err != nil {
		return nil, err
	}

	return &carePlan, nil
}

// handleSearchCarePlan does a search for CarePlan based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCarePlan(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	_, bundle, err := handleSearchResource[fhir.CarePlan](ctx, s, "CarePlan", queryParams, headers)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}
