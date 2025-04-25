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

func (s *Service) handleUpdateQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating QuestionnaireResponse: %s", request.RequestUrl)

	var questionnaireResponse fhir.QuestionnaireResponse
	if err := json.Unmarshal(request.ResourceData, &questionnaireResponse); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", questionnaireResponse, coolfhir.BadRequestError(err))
	}

	// Log the parsed QuestionnaireResponse for debugging
	responseJSON, _ := json.Marshal(questionnaireResponse)
	log.Ctx(ctx).Debug().Msgf("questionnaireResponse: %s", string(responseJSON))

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &questionnaireResponse); err != nil {
		return nil, err
	}

	// Search for the existing QuestionnaireResponse
	var searchBundle fhir.Bundle
	questionnaireResponseId := ""
	if questionnaireResponse.Id != nil {
		questionnaireResponseId = *questionnaireResponse.Id
	}

	if questionnaireResponseId != "" {
		err := s.fhirClient.SearchWithContext(ctx, "QuestionnaireResponse", url.Values{
			"_id": []string{questionnaireResponseId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for QuestionnaireResponse: %w", err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 || questionnaireResponseId == "" {
		log.Ctx(ctx).Info().Msgf("QuestionnaireResponse not found, 	andling as create: %s", questionnaireResponseId)
		request.Upsert = true
		return s.handleCreateQuestionnaireResponse(ctx, request, tx)
	}

	// Extract the existing QuestionnaireResponse from the bundle
	var existingQuestionnaireResponse fhir.QuestionnaireResponse
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingQuestionnaireResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing QuestionnaireResponse: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "QuestionnaireResponse", questionnaireResponseId)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of QuestionnaireResponse")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to QuestionnaireResponse", http.StatusForbidden)
	}

	idx := len(tx.Entry)
	questionnaireResponseBundleEntry := request.bundleEntryWithResource(questionnaireResponse)
	tx.AppendEntry(questionnaireResponseBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
	}))

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var updatedQuestionnaireResponse fhir.QuestionnaireResponse
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &questionnaireResponseBundleEntry, &txResult.Entry[idx], &updatedQuestionnaireResponse)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original QuestionnaireResponse that was sent.
			updatedQuestionnaireResponse = questionnaireResponse
		} else if err != nil {
			return nil, nil, err
		}

		return []*fhir.BundleEntry{result}, []any{&updatedQuestionnaireResponse}, nil
	}, nil
}
