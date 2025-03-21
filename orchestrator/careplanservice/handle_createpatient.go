package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreatePatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating Patient")
	var patient fhir.Patient
	if err := json.Unmarshal(request.ResourceData, &patient); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", patient, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &patient); err != nil {
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

	// Create audit event for the creation
	createAuditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionC,
		&fhir.Reference{
			Reference: patientBundleEntry.FullUrl,
			Type:      to.Ptr("Patient"),
		},
		&fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
	)

	// If patient has an ID, treat as PUT operation
	if patient.Id != nil && request.Upsert {
		tx.Append(patient, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "Patient/" + *patient.Id,
		}, nil, coolfhir.WithFullUrl("Patient/"+*patient.Id))
		createAuditEvent.Entity[0].What.Reference = to.Ptr("Patient/" + *patient.Id)
	} else {
		tx.Create(patient, coolfhir.WithFullUrl(*patientBundleEntry.FullUrl))
	}

	patientEntryIdx := len(tx.Entry) - 1
	tx.Create(createAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdPatient fhir.Patient
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[patientEntryIdx], &txResult.Entry[patientEntryIdx], &createdPatient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Patient creation result: %w", err)
		}

		return result, []any{&createdPatient}, nil
	}, nil
}
