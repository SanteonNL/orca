package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

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

	// Search for the existing Patient
	var searchBundle fhir.Bundle
	patientId := ""
	if patient.Id != nil {
		patientId = *patient.Id
	}
	if patientId != "" {
		err := s.fhirClient.SearchWithContext(ctx, "Patient", url.Values{
			"_id": []string{patientId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for Patient: %w", err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 || patientId == "" {
		log.Ctx(ctx).Info().Msgf("Patient not found, handling as create: %s", patientId)
		request.Upsert = true
		return s.handleCreatePatient(ctx, request, tx)
	}

	// Extract the existing Patient from the bundle
	var existingPatient fhir.Patient
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingPatient)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing Patient: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "Patient", patientId)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of Patient")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to Patient", http.StatusForbidden)
	}

	idx := len(tx.Entry)
	// Add to transaction
	patientBundleEntry := request.bundleEntryWithResource(patient)
	tx.AppendEntry(patientBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
	}))

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var updatedPatient fhir.Patient
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &patientBundleEntry, &txResult.Entry[idx], &updatedPatient)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original Patient that was sent.
			updatedPatient = patient
		} else if err != nil {
			return nil, nil, err
		}

		return []*fhir.BundleEntry{result}, []any{&updatedPatient}, nil
	}, nil
}
