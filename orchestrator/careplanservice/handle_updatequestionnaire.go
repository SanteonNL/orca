package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdateQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating Questionnaire: %s", request.RequestUrl)
	var questionnaire fhir.Questionnaire
	if err := json.Unmarshal(request.ResourceData, &questionnaire); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", questionnaire, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &questionnaire); err != nil {
		return nil, err
	}

	var searchBundle fhir.Bundle
	questionnaireId := ""
	if questionnaire.Id != nil {
		questionnaireId = *questionnaire.Id
	}

	if questionnaireId != "" {
		err := s.fhirClient.SearchWithContext(ctx, "Questionnaire", url.Values{
			"_id": []string{questionnaireId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for Questionnaire: %w", err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 || questionnaireId == "" {
		log.Ctx(ctx).Info().Msgf("Questionnaire not found, handling as create: %s", questionnaireId)
		request.Upsert = true
		return s.handleCreateQuestionnaire(ctx, request, tx)
	}

	// As long as the user is authorised, they may update the questionnaire

	var existingQuestionnaire fhir.Questionnaire
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingQuestionnaire)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing Questionnaire: %w", err)
	}

	// Get local identity for audit
	localIdentity, err := s.getLocalIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get local identity: %w", err)
	}

	updateAuditEvent := audit.Event(*localIdentity, fhir.AuditEventActionU,
		&fhir.Reference{
			Reference: to.Ptr("Questionnaire/" + questionnaireId),
			Type:      to.Ptr("Questionnaire"),
		},
		&fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
	)

	questionnaireBundleEntry := request.bundleEntryWithResource(questionnaire)
	tx.AppendEntry(questionnaireBundleEntry)
	idx := len(tx.Entry) - 1
	tx.Create(updateAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedQuestionnaire fhir.Questionnaire
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &questionnaireBundleEntry, &txResult.Entry[idx], &updatedQuestionnaire)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Questionnaire update result: %w", err)
		}

		return result, []any{&updatedQuestionnaire}, nil
	}, nil
}
