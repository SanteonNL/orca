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

func (s *Service) handleUpdateQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating QuestionnaireResponse: %s", request.RequestUrl)
	var questionnaireResponse fhir.QuestionnaireResponse
	if err := json.Unmarshal(request.ResourceData, &questionnaireResponse); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", questionnaireResponse, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &questionnaireResponse); err != nil {
		return nil, err
	}

	// Get the current principal
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Search for the existing QuestionnaireResponse
	var searchBundle fhir.Bundle
	err = s.fhirClient.SearchWithContext(ctx, "QuestionnaireResponse", url.Values{
		"_id": []string{*questionnaireResponse.Id},
	}, &searchBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to search for QuestionnaireResponse: %w", err)
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		log.Ctx(ctx).Info().Msgf("QuestionnaireResponse not found, handling as create: %s", *questionnaireResponse.Id)
		return s.handleCreateQuestionnaireResponse(ctx, request, tx)
	}

	// Extract the existing QuestionnaireResponse from the bundle
	var existingQuestionnaireResponse fhir.QuestionnaireResponse
	err = json.Unmarshal(searchBundle.Entry[0].Resource, &existingQuestionnaireResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing QuestionnaireResponse: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, "QuestionnaireResponse", *questionnaireResponse.Id)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of QuestionnaireResponse")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to QuestionnaireResponse", http.StatusForbidden)
	}

	// Get local identity for audit
	localIdentity, err := s.getLocalIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get local identity: %w", err)
	}

	// Create audit event for the update
	updateAuditEvent := audit.Event(*localIdentity, fhir.AuditEventActionU,
		&fhir.Reference{
			Reference: to.Ptr("QuestionnaireResponse/" + *questionnaireResponse.Id),
			Type:      to.Ptr("QuestionnaireResponse"),
		},
		&fhir.Reference{
			Identifier: &principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
	)
	// Add to transaction
	questionnaireResponseBundleEntry := request.bundleEntryWithResource(questionnaireResponse)
	tx.AppendEntry(questionnaireResponseBundleEntry)
	idx := len(tx.Entry) - 1
	tx.Create(updateAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedQuestionnaireResponse fhir.QuestionnaireResponse
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &questionnaireResponseBundleEntry, &txResult.Entry[idx], &updatedQuestionnaireResponse)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process QuestionnaireResponse update result: %w", err)
		}

		return result, []any{&updatedQuestionnaireResponse}, nil
	}, nil
}
