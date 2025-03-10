package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
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

	// Questionnaire.id must be generated by the backing FHIR API, so it must be nil in the request
	questionnaire.Id = nil

	// Get the current principal - just need to verify the user is authenticated
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	localIdentity, err := s.getLocalIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get local identity: %w", err)
	}

	// TODO: Field validation

	questionnaireBundleEntry := request.bundleEntryWithResource(questionnaire)
	if questionnaireBundleEntry.FullUrl == nil {
		questionnaireBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	createAuditEvent := audit.Event(*localIdentity, fhir.AuditEventActionC,
		&fhir.Reference{
			Reference: questionnaireBundleEntry.FullUrl,
			Type:      to.Ptr("Questionnaire"),
		},
		&fhir.Reference{
			Identifier: &principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
	)

	tx.Create(questionnaire, coolfhir.WithFullUrl(*questionnaireBundleEntry.FullUrl))
	questionnaireEntryIdx := len(tx.Entry) - 1
	tx.Create(createAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdQuestionnaire fhir.Questionnaire
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[questionnaireEntryIdx], &txResult.Entry[questionnaireEntryIdx], &createdQuestionnaire)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Questionnaire creation result: %w", err)
		}

		return result, []any{&createdQuestionnaire}, nil
	}, nil
}
