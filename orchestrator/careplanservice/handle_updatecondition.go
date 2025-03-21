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

func (s *Service) handleUpdateCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating Condition: %s", request.RequestUrl)
	var condition fhir.Condition
	if err := json.Unmarshal(request.ResourceData, &condition); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", condition, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &condition); err != nil {
		return nil, err
	}

	// Search for the existing Condition
	var searchBundle fhir.Bundle

	conditionId := ""
	if condition.Id != nil {
		conditionId = *condition.Id
	}

	if conditionId != "" {
		err := s.fhirClient.SearchWithContext(ctx, "Condition", url.Values{
			"_id": []string{conditionId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for Condition: %w", err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		log.Ctx(ctx).Info().Msgf("Condition not found, handling as create: %s", conditionId)
		request.Upsert = true
		return s.handleCreateCondition(ctx, request, tx)
	}

	// Extract the existing Condition from the bundle
	var existingCondition fhir.Condition
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingCondition)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing Condition: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "Condition", conditionId)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of Condition")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to Condition", http.StatusForbidden)
	}

	// Add to transaction
	conditionBundleEntry := request.bundleEntryWithResource(condition)
	tx.AppendEntry(conditionBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedCondition fhir.Condition
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &conditionBundleEntry, &txResult.Entry[0], &updatedCondition)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original Condition that was sent.
			updatedCondition = condition
		} else if err != nil {
			// Other error
			return nil, nil, err
		}

		return result, []any{&updatedCondition}, nil
	}, nil
}
