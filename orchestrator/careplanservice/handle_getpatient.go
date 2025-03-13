package careplanservice

import (
	"context"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetPatient fetches the requested Patient and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the patient)
// if the requester is valid, return the Patient, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetPatient(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Patient, error) {
	var patient fhir.Patient
	err := s.fhirClient.ReadWithContext(ctx, "Patient/"+id, &patient, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	authorisedPatients, err := s.filterAuthorizedPatients(ctx, principal, []fhir.Patient{patient})
	if err != nil {
		return nil, err
	}
	if len(authorisedPatients) == 0 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Patient",
			StatusCode: http.StatusForbidden,
		}
	}

	return &patient, nil
}

// handleSearchPatient does a search for Patient based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchPatient(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	patients, bundle, err := handleSearchResource[fhir.Patient](ctx, s, "Patient", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(patients) == 0 {
		// If there are no patients in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	authorisedPatients, err := s.filterAuthorizedPatients(ctx, principal, patients)
	if err != nil {
		return nil, err
	}

	var patientRefs []string
	for _, patient := range authorisedPatients {
		patientRefs = append(patientRefs, "Patient/"+*patient.Id)
	}

	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"Patient"}, patientRefs)

	return &retBundle, nil
}
