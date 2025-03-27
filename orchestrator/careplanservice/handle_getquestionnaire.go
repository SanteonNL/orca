package careplanservice

import (
	"context"
	"fmt"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleGetQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Questionnaire with ID: %s", request.ResourceId)
	var questionnaire fhir.Questionnaire
	err := s.fhirClient.ReadWithContext(ctx, "Questionnaire/"+request.ResourceId, &questionnaire, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	tx.Get(questionnaire, "Questionnaire/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	questionnaireEntryIdx := 0

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var retQuestionnaire fhir.Questionnaire
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[questionnaireEntryIdx], &txResult.Entry[questionnaireEntryIdx], &retQuestionnaire)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Questionnaire read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{result}, []any{}, nil
	}, nil
}
