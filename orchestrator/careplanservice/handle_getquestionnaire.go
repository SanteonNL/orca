package careplanservice

import (
	"context"
	"fmt"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleGetQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Str("questionnaireId", request.ResourceId).Msg("Handling get questionnaire request")

	var questionnaire fhir.Questionnaire
	err := s.fhirClient.ReadWithContext(ctx, "Questionnaire/"+request.ResourceId, &questionnaire)
	if err != nil {
		return nil, err
	}

	tx.Get(questionnaire, "Questionnaire/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionR,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var returnedQuestionnaire fhir.Questionnaire
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[0], &txResult.Entry[0], &returnedQuestionnaire)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get Questionnaire: %w", err)
		}

		return result, []any{&returnedQuestionnaire}, nil
	}, nil
}
