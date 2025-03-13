package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// filterAuthorizedPatients will go through a list of patients and return the ones the requester has access to
func (s *Service) filterAuthorizedPatients(ctx context.Context, patients []fhir.Patient) ([]fhir.Patient, error) {
	params := url.Values{}
	patientRefs := make([]string, len(patients))
	for i, patient := range patients {
		patientRefs[i] = fmt.Sprintf("Patient/%s", *patient.Id)
	}
	params.Add("subject", strings.Join(patientRefs, ","))

	// Fetch all CarePlans associated with the Patient, get the CareTeams associated with the CarePlans
	// Get the CarePlan for which the Patient is the subject, get the CareTeams associated with the CarePlan
	var verificationBundle fhir.Bundle
	err := s.fhirClient.SearchWithContext(ctx, "CarePlan", params, &verificationBundle)
	if err != nil {
		return nil, err
	}

	var carePlans []fhir.CarePlan
	err = coolfhir.ResourcesInBundle(&verificationBundle, coolfhir.EntryIsOfType("CarePlan"), &carePlans)
	if err != nil {
		return nil, err
	}

	retPatients := make([]fhir.Patient, 0)

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Iterate through each CareTeam to see if the requester is a participant, if not, remove any patients from the bundle that are part of the CareTeam
	for _, cp := range carePlans {
		ct, err := coolfhir.CareTeamFromCarePlan(&cp)
		// INT-630: We changed CareTeam to be contained within the CarePlan, but old test data in the CarePlan resource does not have CareTeam.
		//          For temporary backwards compatibility, ignore these CarePlans. It can be removed when old data has been purged.
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Unable to derive CareTeam from CarePlan, ignoring CarePlan for authorizing access to FHIR Patient resource (carePlanID=%s)", *cp.Id)
			continue
		}

		participant := coolfhir.FindMatchingParticipantInCareTeam(ct, principal.Organization.Identifier)
		if participant != nil {
			for _, patient := range patients {
				if "Patient/"+*patient.Id == *cp.Subject.Reference {
					retPatients = append(retPatients, patient)
					break
				}
			}
		}
	}

	return retPatients, nil
}

// handleSearchResource is a generic function to search for a resource of a given type and return the results
// it returns a processed list of the required resource type, the full bundle and an error
func handleSearchResource[T any](ctx context.Context, s *Service, resourceType string, queryParams url.Values, headers *fhirclient.Headers) ([]T, *fhir.Bundle, error) {
	form := url.Values{}
	for k, v := range queryParams {
		for _, value := range v {
			form.Add(k, value)
		}
	}

	var bundle fhir.Bundle
	err := s.fhirClient.SearchWithContext(ctx, resourceType, form, &bundle, fhirclient.ResponseHeaders(headers))
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

// filterMatchingResourcesInBundle will find all resources in the bundle of the given type with a matching ID and return a new bundle with only those resources
// To populate the 'total' field, the function will count the number of matching resources that
func filterMatchingResourcesInBundle(ctx context.Context, bundle *fhir.Bundle, resourceTypes []string, references []string) fhir.Bundle {
	newBundle := *bundle
	j := 0
	for _, entry := range newBundle.Entry {
		var resourceInBundle coolfhir.Resource
		err := json.Unmarshal(entry.Resource, &resourceInBundle)
		if err != nil {
			// We don't want to fail the whole operation if one resource fails to unmarshal.
			// Replace result bundle entry with an OperationOutcome to inform the client something went wrong.
			log.Ctx(ctx).Error().Msgf("filterMatchingResourcesInBundle: Failed to unmarshal resource: %v", err)
			newBundle.Entry[j] = coolfhir.CreateOperationOutcomeBundleEntryFromError(err, "Failed to unmarshal resource")
			j++
			continue
		}

		if slices.Contains(resourceTypes, resourceInBundle.Type) {
			for _, ref := range references {
				parts := strings.Split(ref, "/")
				if len(parts) != 2 {
					// Replace result bundle entry with an OperationOutcome, since we couldn't resolve it
					log.Ctx(ctx).Error().Msgf("filterMatchingResourcesInBundle: Invalid reference format: %s", ref)
					newBundle.Entry[j] = coolfhir.CreateOperationOutcomeBundleEntryFromError(fmt.Errorf("Invalid reference format: %s", ref), "Invalid reference format")
					j++
					continue
				}
				if parts[0] == resourceInBundle.Type && parts[1] == resourceInBundle.ID {
					newBundle.Entry[j] = entry
					j++
					break
				}
			}
		}
	}
	newBundle.Entry = newBundle.Entry[:j]
	return newBundle
}
