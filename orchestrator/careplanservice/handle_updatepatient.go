package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdatePatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating Patient: %s", request.RequestUrl)
	var patient fhir.Patient
	if err := json.Unmarshal(request.ResourceData, &patient); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", patient, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &patient); err != nil {
		return nil, err
	}

	// Get the current principal
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Search for the existing Patient
	var searchBundle fhir.Bundle
	err = s.fhirClient.SearchWithContext(ctx, "Patient", url.Values{
		"_id": []string{*patient.Id},
	}, &searchBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to search for Patient: %w", err)
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		log.Ctx(ctx).Info().Msgf("Patient not found, handling as create: %s", *patient.Id)
		return s.handleCreatePatient(ctx, request, tx)
	}

	// Extract the existing Patient from the bundle
	var existingPatient fhir.Patient
	err = json.Unmarshal(searchBundle.Entry[0].Resource, &existingPatient)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing Patient: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, "Patient", *patient.Id)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of Patient")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to Patient", http.StatusForbidden)
	}

	// Get local identity for audit
	localIdentity, err := s.getLocalIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get local identity: %w", err)
	}

	// Create audit event for the update
	updateAuditEvent := audit.Event(*localIdentity, fhir.AuditEventActionU,
		&fhir.Reference{
			Reference: to.Ptr("Patient/" + *patient.Id),
			Type:      to.Ptr("Patient"),
		},
		&fhir.Reference{
			Identifier: &principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
	)

	// Add to transaction
	patientBundleEntry := request.bundleEntryWithResource(patient)
	tx.AppendEntry(patientBundleEntry)
	idx := len(tx.Entry) - 1
	tx.Create(updateAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedPatient fhir.Patient
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &patientBundleEntry, &txResult.Entry[idx], &updatedPatient)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Patient update result: %w", err)
		}

		return result, []any{&updatedPatient}, nil
	}, nil
}
