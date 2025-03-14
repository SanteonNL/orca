package careplanservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetPatient fetches the requested Patient and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the patient)
// if the requester is valid, return the Patient, else return an error
func (s *Service) handleGetPatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Str("patientId", request.ResourceId).Msg("Handling get patient request")

	var patient fhir.Patient
	err := s.fhirClient.ReadWithContext(ctx, "Patient/"+request.ResourceId, &patient)
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

	tx.Get(patient, "Patient/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionR,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var returnedPatient fhir.Patient
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[0], &txResult.Entry[0], &returnedPatient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get Patient: %w", err)
		}

		return result, []any{&returnedPatient}, nil
	}, nil
}

// handleSearchPatient does a search for Patient based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchPatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	patients, bundle, err := handleSearchResource[fhir.Patient](ctx, s, "Patient", request.QueryParams, request.FHIRHeaders)
	if err != nil {
		return nil, err
	}

	var retBundle fhir.Bundle

	// If there are no patients in the bundle there is no point in doing validation, return empty bundle to user
	// We still need to return a bundle and create an audit event
	if len(patients) > 0 {
		authorisedPatients, err := s.filterAuthorizedPatients(ctx, *request.Principal, patients)
		if err != nil {
			return nil, err
		}

		var patientRefs []string
		for _, patient := range authorisedPatients {
			patientRefs = append(patientRefs, "Patient/"+*patient.Id)
		}

		retBundle = filterMatchingResourcesInBundle(ctx, bundle, []string{"Patient"}, patientRefs)
	}

	for _, entry := range retBundle.Entry {
		tx.Get(entry.Resource, entry.Request.Url, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer:    *request.LocalIdentity,
			Action:      fhir.AuditEventActionR,
			QueryParams: request.QueryParams,
		}))
	}

	// tx.Get(retBundle, "Patient", coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
	// 	ActingAgent: &fhir.Reference{
	// 		Identifier: request.LocalIdentity,
	// 		Type:       to.Ptr("Organization"),
	// 	},
	// 	Observer: *request.LocalIdentity,
	// 	Action:   fhir.AuditEventActionR,
	// }))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[0], &txResult.Entry[0], &retBundle)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			return &fhir.BundleEntry{
				Link: []fhir.BundleLink{
					{
						Relation: "self",
						Url:      s.fhirURL.String() + "/Patient",
					},
				},
			}, []any{&retBundle}, nil
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get Patient: %w", err)
		}

		return result, []any{&retBundle}, nil
	}, nil
}
