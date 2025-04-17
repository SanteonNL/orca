package careplanservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleReadQuestionnaireResponse fetches the requested QuestionnaireResponse and validates if the requester has access
func (s *Service) handleReadQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
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

// handleSearchQuestionnaireResponse performs a search for QuestionnaireResponse based on the user request parameters
// and filters the results based on user authorization
func (s *Service) handleSearchQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for QuestionnaireResponses")

	bundle, err := s.searchQuestionnaireResponse(ctx, request.QueryParams, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}

	for _, entry := range bundle.Entry {
		var currentQuestionnaireResponse fhir.QuestionnaireResponse
		if err := json.Unmarshal(entry.Resource, &currentQuestionnaireResponse); err != nil {
			log.Ctx(ctx).Error().
				Err(err).
				Msg("Failed to unmarshal resource for audit")
			continue
		}

		// Create the query detail entity
		queryEntity := fhir.AuditEventEntity{
			Type: &fhir.Coding{
				System:  to.Ptr("http://terminology.hl7.org/CodeSystem/audit-entity-type"),
				Code:    to.Ptr("2"), // query parameters
				Display: to.Ptr("Query Parameters"),
			},
			Detail: []fhir.AuditEventEntityDetail{},
		}

		// Add each query parameter as a detail
		for param, values := range request.QueryParams {
			queryEntity.Detail = append(queryEntity.Detail, fhir.AuditEventEntityDetail{
				Type:        param, // parameter name as string
				ValueString: to.Ptr(strings.Join(values, ",")),
			})
		}

		bundleEntry := fhir.BundleEntry{
			Resource: entry.Resource,
			Response: &fhir.BundleEntryResponse{
				Status: "200 OK",
			},
		}
		results = append(results, &bundleEntry)

		// Add audit event to the transaction
		auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
			Id:        currentQuestionnaireResponse.Id,
			Type:      to.Ptr("QuestionnaireResponse"),
			Reference: to.Ptr("QuestionnaireResponse/" + *currentQuestionnaireResponse.Id),
		}, &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
		tx.Create(auditEvent)
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// Simply return the already prepared results
		return results, []any{}, nil
	}, nil
}

// searchQuestionnaireResponse performs the core functionality of searching for questionnaire responses and filtering by authorization
// This can be used by other resources to search for questionnaire responses and filter by authorization
func (s *Service) searchQuestionnaireResponse(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers, principal auth.Principal) (*fhir.Bundle, error) {
	questionnaireResponses, bundle, err := handleSearchResource[fhir.QuestionnaireResponse](ctx, s, "QuestionnaireResponse", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(questionnaireResponses) == 0 {
		// If there are no questionnaire responses in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	// For each QuestionnaireResponse, check if the user has access via Task or as creator
	var allowedQuestionnaireResponseRefs []string
	for _, qr := range questionnaireResponses {
		// Try to find a Task that references this QuestionnaireResponse in output
		taskBundle, err := s.searchTask(ctx, url.Values{"output-reference": []string{"QuestionnaireResponse/" + *qr.Id}}, headers, principal)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking tasks for QuestionnaireResponse/%s", *qr.Id)
			continue
		}

		// If task is found, the user has access
		if len(taskBundle.Entry) > 0 {
			allowedQuestionnaireResponseRefs = append(allowedQuestionnaireResponseRefs, "QuestionnaireResponse/"+*qr.Id)
			continue
		}

		// If no task is found, check if the user is the creator
		isCreator, err := s.isCreatorOfResource(ctx, principal, "QuestionnaireResponse", *qr.Id)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if user is creator of QuestionnaireResponse/%s", *qr.Id)
			continue
		}

		if isCreator {
			allowedQuestionnaireResponseRefs = append(allowedQuestionnaireResponseRefs, "QuestionnaireResponse/"+*qr.Id)
		}
	}

	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"QuestionnaireResponse"}, allowedQuestionnaireResponseRefs)

	return &retBundle, nil
}
