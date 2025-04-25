package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating Questionnaire")
	var questionnaire fhir.Questionnaire
	if err := json.Unmarshal(request.ResourceData, &questionnaire); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", questionnaire, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &questionnaire); err != nil {
		return nil, err
	}

	// TODO: Field validation

	questionnaireBundleEntry := request.bundleEntryWithResource(questionnaire)
	if questionnaireBundleEntry.FullUrl == nil {
		questionnaireBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	idx := len(tx.Entry)
	// If questionnaire has an ID, treat as PUT operation
	if questionnaire.Id != nil && request.Upsert {
		tx.Append(questionnaire, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "Questionnaire/" + *questionnaire.Id,
		}, nil, coolfhir.WithFullUrl(*questionnaireBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	} else {
		tx.Create(questionnaire, coolfhir.WithFullUrl(*questionnaireBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var createdQuestionnaire fhir.Questionnaire
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdQuestionnaire)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Questionnaire creation result: %w", err)
		}

		return []*fhir.BundleEntry{result}, []any{&createdQuestionnaire}, nil
	}, nil
}
