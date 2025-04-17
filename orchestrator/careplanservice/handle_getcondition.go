package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
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

// handleReadCondition fetches the requested Condition and validates if the requester has access to the resource
// by checking if they have access to the Patient referenced in the Condition's subject
// if the requester is valid, return the Condition, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleReadCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Condition with ID: %s", request.ResourceId)
	var condition fhir.Condition
	err := s.fhirClient.ReadWithContext(ctx, "Condition/"+request.ResourceId, &condition, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: Find out new auth requirements for condition
	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier != nil && condition.Subject.Identifier.System != nil && condition.Subject.Identifier.Value != nil {
		bundle, err := s.searchPatient(ctx, map[string][]string{"identifier": {fmt.Sprintf("%s|%s", *condition.Subject.Identifier.System, *condition.Subject.Identifier.Value)}}, request.FhirHeaders, *request.Principal)
		if err != nil {
			return nil, err
		}
		if len(bundle.Entry) == 0 {
			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			}
		}
	} else {
		log.Ctx(ctx).Warn().Msg("Condition does not have Patient as subject, can't verify access")
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Condition",
			StatusCode: http.StatusForbidden,
		}
	}

	conditionRaw, err := json.Marshal(condition)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: conditionRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        condition.Id,
		Type:      to.Ptr("Condition"),
		Reference: to.Ptr("Condition/" + *condition.Id),
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

// handleSearchCondition performs a search for Condition based on the user request parameters
// and filters the results based on user authorization
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for Conditions")

	bundle, err := s.searchCondition(ctx, request.QueryParams, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}

	for _, entry := range bundle.Entry {
		var currentCondition fhir.Condition
		if err := json.Unmarshal(entry.Resource, &currentCondition); err != nil {
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
			Id:        currentCondition.Id,
			Type:      to.Ptr("Condition"),
			Reference: to.Ptr("Condition/" + *currentCondition.Id),
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

// searchCondition performs the core functionality of searching for conditions and filtering by authorization
// This can be used by other resources to search for conditions and filter by authorization
func (s *Service) searchCondition(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers, principal auth.Principal) (*fhir.Bundle, error) {
	conditions, bundle, err := handleSearchResource[fhir.Condition](ctx, s, "Condition", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(conditions) == 0 {
		// If there are no conditions in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	// For each Condition, verify the user has access to the patient referenced in the subject
	var allowedConditionRefs []string
	for _, cond := range conditions {
		// Verify if the Condition has a valid Patient subject and the user has access to this patient
		if cond.Subject.Identifier != nil && cond.Subject.Identifier.System != nil && cond.Subject.Identifier.Value != nil {
			patientBundle, err := s.searchPatient(ctx, map[string][]string{"identifier": {fmt.Sprintf("%s|%s", *cond.Subject.Identifier.System, *cond.Subject.Identifier.Value)}}, headers, principal)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msgf("Error checking patient access for Condition/%s", *cond.Id)
				continue
			}

			// If patient is found and user has access, the user also has access to the condition
			if len(patientBundle.Entry) > 0 {
				allowedConditionRefs = append(allowedConditionRefs, "Condition/"+*cond.Id)
				continue
			}
		}

		// If user doesn't have access via patient, check if they are the creator of the condition
		isCreator, err := s.isCreatorOfResource(ctx, principal, "Condition", *cond.Id)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if user is creator of Condition/%s", *cond.Id)
			continue
		}

		if isCreator {
			allowedConditionRefs = append(allowedConditionRefs, "Condition/"+*cond.Id)
		}
	}

	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"Condition"}, allowedConditionRefs)

	return &retBundle, nil
}
