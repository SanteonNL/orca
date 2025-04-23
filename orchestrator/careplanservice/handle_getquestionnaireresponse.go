package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

func ReadQuestionnaireResponseAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.QuestionnaireResponse] {
	return AnyMatchPolicy[fhir.QuestionnaireResponse]{
		Policies: []Policy[fhir.QuestionnaireResponse]{
			RelatedResourceSearchPolicy[fhir.QuestionnaireResponse, fhir.Task]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadTaskAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.QuestionnaireResponse) (resourceType string, searchParams url.Values) {
					return "Task", url.Values{"output-reference": []string{"QuestionnaireResponse/" + *resource.Id}}
				},
			},
			CreatorPolicy[fhir.QuestionnaireResponse]{},
		},
	}
}

// handleReadQuestionnaireResponse fetches the requested QuestionnaireResponse and validates if the requester has access
func (s *Service) handleReadQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting QuestionnaireResponse with ID: %s", request.ResourceId)
	var questionnaireResponse fhir.QuestionnaireResponse
	err := s.fhirClient.ReadWithContext(ctx, "QuestionnaireResponse/"+request.ResourceId, &questionnaireResponse, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	canAccess, err := ReadQuestionnaireResponseAuthzPolicy(s.fhirClient).HasAccess(ctx, questionnaireResponse, *request.Principal)
	if !canAccess {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of QuestionnaireResponse")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to QuestionnaireResponse",
			StatusCode: http.StatusForbidden,
		}
	}

	questionnaireResponseRaw, err := json.Marshal(questionnaireResponse)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: questionnaireResponseRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        questionnaireResponse.Id,
		Type:      to.Ptr("QuestionnaireResponse"),
		Reference: to.Ptr("QuestionnaireResponse/" + *questionnaireResponse.Id),
	}, &fhir.Reference{
		Identifier: &request.Principal.Organization.Identifier[0],
		Type:       to.Ptr("Organization"),
	})
	tx.Create(auditEvent)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}
