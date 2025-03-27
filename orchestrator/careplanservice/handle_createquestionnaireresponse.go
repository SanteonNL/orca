package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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

	// Verify the requester is the same as the local identity
	if !isRequesterLocalCareOrganization([]fhir.Organization{{Identifier: []fhir.Identifier{*request.LocalIdentity}}}, *request.Principal) {
		return nil, coolfhir.NewErrorWithCode("Only the local care organization can create a QuestionnaireResponse", http.StatusForbidden)
	}

	// TODO: Validate required fields

	questionnaireResponseBundleEntry := request.bundleEntryWithResource(questionnaireResponse)
	if questionnaireResponseBundleEntry.FullUrl == nil {
		questionnaireResponseBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	idx := len(tx.Entry)
	// If questionnaireResponse has an ID, treat as PUT operation
	if questionnaireResponse.Id != nil && request.Upsert {
		tx.Append(questionnaireResponse, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "QuestionnaireResponse/" + *questionnaireResponse.Id,
		}, nil, coolfhir.WithFullUrl(*questionnaireResponseBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	} else {
		tx.Create(questionnaireResponse, coolfhir.WithFullUrl(*questionnaireResponseBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdQuestionnaireResponse fhir.QuestionnaireResponse
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdQuestionnaireResponse)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process QuestionnaireResponse creation result: %w", err)
		}

		return result, []any{&createdQuestionnaireResponse}, nil
	}, nil
}
