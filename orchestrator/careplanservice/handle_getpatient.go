package careplanservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleReadPatient fetches the requested Patient and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the patient)
// if the requester is valid, return the Patient, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleReadPatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Patient with ID: %s", request.ResourceId)
	var patient fhir.Patient
	err := s.fhirClient.ReadWithContext(ctx, "Patient/"+request.ResourceId, &patient, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	authorisedPatients, err := s.filterAuthorizedPatients(ctx, *request.Principal, []fhir.Patient{patient})
	if err != nil {
		return nil, err
	}
	if len(authorisedPatients) == 0 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Patient",
			StatusCode: http.StatusForbidden,
		}
	}

	patientRaw, err := json.Marshal(patient)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: patientRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        patient.Id,
		Type:      to.Ptr("Patient"),
		Reference: to.Ptr("Patient/" + *patient.Id),
	}, &fhir.Reference{
		Identifier: &request.Principal.Organization.Identifier[0],
		Type:       to.Ptr("Organization"),
	})
	tx.Create(auditEvent)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}

// handleSearchPatient does a search for Patient based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchPatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for Patients")

	bundle, err := s.searchPatient(ctx, request.QueryParams, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}

	for _, entry := range bundle.Entry {
		var currentPatient fhir.Patient
		if err := json.Unmarshal(entry.Resource, &currentPatient); err != nil {
			log.Ctx(ctx).Error().
				Err(err).
				Msg("Failed to unmarshal resource for audit")
			continue
		}

		// Create the query detail entity
		queryEntity := fhir.AuditEventEntity{
			Type: &fhir.Coding{
				System:  to.Ptr("http://terminology.hl7.org/CodeSystem/audit-entity-type"),
				Code:    to.Ptr("2"), // query parameters
				Display: to.Ptr("Query Parameters"),
			},
			Detail: []fhir.AuditEventEntityDetail{},
		}

		// Add each query parameter as a detail
		for param, values := range request.QueryParams {
			queryEntity.Detail = append(queryEntity.Detail, fhir.AuditEventEntityDetail{
				Type:        param, // parameter name as string
				ValueString: to.Ptr(strings.Join(values, ",")),
			})
		}

		bundleEntry := fhir.BundleEntry{
			Resource: entry.Resource,
			Response: &fhir.BundleEntryResponse{
				Status: "200 OK",
			},
		}
		results = append(results, &bundleEntry)

		// Add audit event to the transaction
		auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
			Id:        currentPatient.Id,
			Type:      to.Ptr("Patient"),
			Reference: to.Ptr("Patient/" + *currentPatient.Id),
		}, &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
		tx.Create(auditEvent)
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// Simply return the already prepared results
		return results, []any{}, nil
	}, nil
}

// searchPatient performs the core functionality of searching for patients and filtering by authorization
// This can be used by other resources to search for patients and filter by authorization
func (s *Service) searchPatient(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers, principal auth.Principal) (*fhir.Bundle, error) {
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
