package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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

// TODO: searchCareTeam once settled on searchCarePlan logic
