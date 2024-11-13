package careplanservice

import (
	"context"
	"encoding/json"
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
	// fetch Patient + CareTeam, validate requester is participant of CareTeam
	var patient fhir.Patient
	err := s.fhirClient.Read("Patient/"+id, &patient, fhirclient.ResponseHeaders(headers))
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

// handleSearchPatient does a search for Patient based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchPatient(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	params := []fhirclient.Option{}
	for k, v := range queryParams {
		for _, value := range v {
			params = append(params, fhirclient.QueryParam(k, value))
		}
	}

	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read("Patient", &bundle, params...)
	if err != nil {
		return nil, err
	}

	var patients []fhir.Patient
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Patient"), &patients)
	if err != nil {
		return nil, err
	}
	if len(patients) == 0 {
		// If there are no patients in the bundle there is no point in doing validation, return empty bundle to user
		return &bundle, nil
	}

	// It is possible that we have patients that are part of different CarePlans. Create list of query params for each patient ID
	params = []fhirclient.Option{}
	BSNs := ""
	for i, patient := range patients {
		if i == 0 {
			BSNs = patientBSN(&patient)
		} else {
			BSNs = fmt.Sprintf("%s,%s", BSNs, patientBSN(&patient))
		}
	}
	params = append(params, fhirclient.QueryParam("subject-identifier", BSNs))
	params = append(params, fhirclient.QueryParam("_include", "CarePlan:care-team"))

	// Fetch all CarePlans associated with the Patient, get the CareTeams associated with the CarePlans
	// Get the CarePlan for which the Patient is the subject, get the CareTeams associated with the CarePlan
	var verificationBundle fhir.Bundle
	err = s.fhirClient.Read("CarePlan", &verificationBundle, params...)

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(&verificationBundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return nil, err
	}
	var carePlans []fhir.CarePlan
	err = coolfhir.ResourcesInBundle(&verificationBundle, coolfhir.EntryIsOfType("CarePlan"), &carePlans)
	if err != nil {
		return nil, err
	}

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: This is messy, see if it can be simplified and made generic enough to use to filter other resource types
	// Iterate through each CareTeam to see if the requester is a participant, if not, remove any patients from the bundle that are part of the CareTeam
	for _, cp := range carePlans {
		var ct fhir.CareTeam
		for _, c := range careTeams {
			if *cp.CareTeam[0].Reference == fmt.Sprintf("CareTeam/%s", *c.Id) {
				ct = c
				break
			}
		}
		participant := coolfhir.FindMatchingParticipantInCareTeam([]fhir.CareTeam{ct}, principal.Organization.Identifier)
		if participant == nil {
			for i, patient := range patients {
				removePatient := true
				if patientBSN(&patient) != "" {
					if patientBSN(&patient) == *cp.Subject.Reference {
						removePatient = false
						break
					}
				}
				if removePatient {
					patients = append(patients[:i], patients[i+1:]...)
				}
			}
		}
	}
	// Iterate through remaining patients, if they have no CareTeam, remove them from the bundle
	for i, patient := range patients {
		removePatient := true
		if patientBSN(&patient) != "" {
			for _, cp := range carePlans {
				if patientBSN(&patient) == fmt.Sprintf("%s|%s", *cp.Subject.Identifier.System, *cp.Subject.Identifier.Value) {
					removePatient = false
					break
				}
			}
		}
		if removePatient {
			patients = append(patients[:i], patients[i+1:]...)
		}
	}

	// Remove patients from bundle that are not in the new patient list
	for i, entry := range bundle.Entry {
		removePatient := true
		for _, p := range patients {
			var resource coolfhir.Resource
			err = json.Unmarshal(entry.Resource, &resource)
			if err != nil {
				return nil, err
			}

			if resource.ID == *p.Id {
				removePatient = false
				break
			}
		}
		if removePatient {
			bundle.Entry = append(bundle.Entry[:i], bundle.Entry[i+1:]...)
		}
	}

	return &bundle, nil
}

func patientBSN(patient *fhir.Patient) string {
	for _, id := range patient.Identifier {
		if id.System != nil && *id.System == "http://fhir.nl/fhir/NamingSystem/bsn" {
			return fmt.Sprintf("%s|%s", *id.System, *id.Value)
		}
	}
	return ""
}
