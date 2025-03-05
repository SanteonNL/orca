package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating QuestionnaireResponse")
	var questionnaireResponse fhir.QuestionnaireResponse
	if err := json.Unmarshal(request.ResourceData, &questionnaireResponse); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", questionnaireResponse, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &questionnaireResponse); err != nil {
		return nil, err
	}

	// QuestionnaireResponse.id must be generated by the backing FHIR API, so it must be nil in the request
	questionnaireResponse.Id = nil

	// Get the current principal
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Get local identity for audit
	localIdentity, err := s.getLocalIdentity()
	if err != nil {
		return nil, fmt.Errorf("failed to get local identity: %w", err)
	}

	// Verify the requester is the same as the local identity
	if !isRequesterLocalCareOrganization([]fhir.Organization{{Identifier: []fhir.Identifier{*localIdentity}}}, principal) {
		return nil, coolfhir.NewErrorWithCode("Only the local care organization can create a QuestionnaireResponse", http.StatusForbidden)
	}

	// TODO: Validate required fields

	questionnaireResponseBundleEntry := request.bundleEntryWithResource(questionnaireResponse)
	if questionnaireResponseBundleEntry.FullUrl == nil {
		questionnaireResponseBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	// Create audit event for the creation
	createAuditEvent := audit.Event(*localIdentity, fhir.AuditEventActionC,
		&fhir.Reference{
			Reference: questionnaireResponseBundleEntry.FullUrl,
			Type:      to.Ptr("QuestionnaireResponse"),
		},
		&fhir.Reference{
			Identifier: &principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
	)

	// Add to transaction
	tx.Create(questionnaireResponse, coolfhir.WithFullUrl(*questionnaireResponseBundleEntry.FullUrl))
	questionnaireResponseEntryIdx := len(tx.Entry) - 1
	tx.Create(createAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdQuestionnaireResponse fhir.QuestionnaireResponse
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[questionnaireResponseEntryIdx], &txResult.Entry[questionnaireResponseEntryIdx], &createdQuestionnaireResponse)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process QuestionnaireResponse creation result: %w", err)
		}

		return result, []any{&createdQuestionnaireResponse}, nil
	}, nil
}
