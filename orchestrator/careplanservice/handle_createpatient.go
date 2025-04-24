package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreatePatientAuthzPolicy(profile profile.Provider) Policy[fhir.Patient] {
	return LocalOrganizationPolicy[fhir.Patient]{
		profile: profile,
	}
}

func (s *Service) handleCreatePatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating Patient")
	var patient fhir.Patient
	if err := json.Unmarshal(request.ResourceData, &patient); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", patient, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := validateLiteralReferences(ctx, s.profile, &patient); err != nil {
		return nil, err
	}

	// Verify the requester is the same as the local identity
	if !isRequesterLocalCareOrganization([]fhir.Organization{{Identifier: []fhir.Identifier{*request.LocalIdentity}}}, *request.Principal) {
		return nil, coolfhir.NewErrorWithCode("Only the local care organization can create a Patient", http.StatusForbidden)
	}

	// TODO: Field validation

	patientBundleEntry := request.bundleEntryWithResource(patient)
	if patientBundleEntry.FullUrl == nil {
		patientBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	idx := len(tx.Entry)
	// If the patient has an ID and the upsert flag is set, treat as PUT operation
	// As per FHIR spec, this is how we can create a resource with a client supplied ID: https://hl7.org/fhir/http.html#upsert
	if patient.Id != nil && request.Upsert {
		tx.Append(patient, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "Patient/" + *patient.Id,
		}, nil, coolfhir.WithFullUrl(*patientBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	} else {
		tx.Create(patient, coolfhir.WithFullUrl(*patientBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var createdPatient fhir.Patient
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdPatient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Patient creation result: %w", err)
		}

		return []*fhir.BundleEntry{result}, []any{&createdPatient}, nil
	}, nil
}
