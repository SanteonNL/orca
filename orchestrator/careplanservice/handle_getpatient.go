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
)

// handleGetPatient fetches the requested Patient and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the patient)
// if the requester is valid, return the Patient, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetPatient(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Patient, error) {
	// Verify requester is authenticated
	_, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var patient fhir.Patient
	err = s.fhirClient.Read("Patient/"+id, &patient, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// Get the CarePlan for which the Patient is the subject, get the CareTeams associated with the CarePlan
	// The search for CarePlan already checks auth for CareTeam, so if we get back a valid CarePlan, we can assume the user has access to the patient
	// If the user is not part of any of these CareTeams, the search for CarePlan will return an error
	bundle, err := s.handleSearchCarePlan(ctx, url.Values{"subject-identifier": []string{patientBSN(&patient)}}, headers)
	if err != nil {
		return nil, err
	}

	var carePlans []fhir.CarePlan
	err = coolfhir.ResourcesInBundle(bundle, coolfhir.EntryIsOfType("CarePlan"), &carePlans)
	if err != nil {
		return nil, err
	}

	// I don't know if this is possible, but worth safeguarding against
	if len(carePlans) == 0 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Patient is not part of any CarePlan, can't verify access",
			StatusCode: http.StatusForbidden,
		}
	}

	return &patient, nil
}

func patientBSN(patient *fhir.Patient) string {
	for _, id := range patient.Identifier {
		if id.System != nil && *id.System == "http://fhir.nl/fhir/NamingSystem/bsn" {
			return fmt.Sprintf("%s|%s", *id.System, *id.Value)
		}
	}
	return ""
}
