package careplanservice

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaireResponse fetches the requested QuestionnaireResponse and validates if the requester has access
func (s *Service) handleGetQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting QuestionnaireResponse with ID: %s", request.ResourceId)
	var questionnaireResponse fhir.QuestionnaireResponse
	err := s.fhirClient.ReadWithContext(ctx, "QuestionnaireResponse/"+request.ResourceId, &questionnaireResponse, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// Fetch tasks where the QuestionnaireResponse is in the task Output
	// If the user has access to the task, they have access to the questionnaire response
	bundle, err := s.searchTask(ctx, url.Values{"output-reference": []string{"QuestionnaireResponse/" + request.ResourceId}}, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	// If the user does not have access to the task, check if they are the creator of the questionnaire response
	if len(bundle.Entry) == 0 {
		// If the user created the questionnaire response, they have access to it
		isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "QuestionnaireResponse", request.ResourceId)
		if isCreator {
			// User has access, continue with transaction
		} else {
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of QuestionnaireResponse")
			}

			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to QuestionnaireResponse",
				StatusCode: http.StatusForbidden,
			}
		}
	}

	tx.Get(questionnaireResponse, "QuestionnaireResponse/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	qrEntryIdx := 0

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var retQR fhir.QuestionnaireResponse
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[qrEntryIdx], &txResult.Entry[qrEntryIdx], &retQR)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process QuestionnaireResponse read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{result}, []any{}, nil
	}, nil
}
