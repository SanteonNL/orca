package careplanservice

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleReadQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleReadQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Questionnaire with ID: %s", request.ResourceId)
	var questionnaire fhir.Questionnaire
	err := s.fhirClient.ReadWithContext(ctx, "Questionnaire/"+request.ResourceId, &questionnaire, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	questionnaireRaw, err := json.Marshal(questionnaire)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: questionnaireRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        questionnaire.Id,
		Type:      to.Ptr("Questionnaire"),
		Reference: to.Ptr("Questionnaire/" + *questionnaire.Id),
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

// handleSearchQuestionnaire performs a search for Questionnaire based on the user request parameters
func (s *Service) handleSearchQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for Questionnaires")

	bundle, err := s.searchQuestionnaire(ctx, request.QueryParams, request.FhirHeaders)
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}

	for _, entry := range bundle.Entry {
		var currentQuestionnaire fhir.Questionnaire
		if err := json.Unmarshal(entry.Resource, &currentQuestionnaire); err != nil {
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
			Id:        currentQuestionnaire.Id,
			Type:      to.Ptr("Questionnaire"),
			Reference: to.Ptr("Questionnaire/" + *currentQuestionnaire.Id),
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

// searchQuestionnaire performs the core functionality of searching for questionnaires
// This can be used by other resources to search for questionnaires
func (s *Service) searchQuestionnaire(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	var bundle fhir.Bundle
	err := s.fhirClient.SearchWithContext(ctx, "Questionnaire", queryParams, &bundle, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	return &bundle, nil
}
