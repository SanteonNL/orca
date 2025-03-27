package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetTask fetches the requested Task and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the task)
// if the requester is valid, return the Task, else return an error
func (s *Service) handleGetTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Task with ID: %s", request.ResourceId)
	// fetch Task + CareTeam, validate requester is participant of CareTeam
	var task fhir.Task
	err := s.fhirClient.ReadWithContext(ctx, "Task/"+request.ResourceId, &task, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}
	// This shouldn't be possible, but still worth checking
	if len(task.BasedOn) != 1 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Task has invalid number of BasedOn values",
			StatusCode: http.StatusInternalServerError,
		}
	}
	if task.BasedOn[0].Reference == nil {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Task has invalid BasedOn Reference",
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Check if the requester is either the task Owner or Requester, if not, they must be a member of the CareTeam
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&task, request.Principal.Organization.Identifier)
	if !(isOwner || isRequester) {
		var carePlan fhir.CarePlan

		if err := s.fhirClient.ReadWithContext(ctx, *task.BasedOn[0].Reference, &carePlan); err != nil {
			return nil, err
		}

		careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
		if err != nil {
			return nil, err
		}

		err = validatePrincipalInCareTeam(*request.Principal, careTeam)
		if err != nil {
			return nil, err
		}
	}

	tx.Get(task, "Task/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	taskEntryIdx := 0

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var retTask fhir.Task
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[taskEntryIdx], &txResult.Entry[taskEntryIdx], &retTask)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Task read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{result}, []any{}, nil
	}, nil
}

// handleSearchTask does a search for Task based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for Tasks")

	bundle, err := s.searchTask(ctx, request.QueryParams, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	taskEntryIndexes := []int{}

	for _, entry := range bundle.Entry {
		var currentTask fhir.Task
		if err := json.Unmarshal(entry.Resource, &currentTask); err != nil {
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

		taskEntryIndexes = append(taskEntryIndexes, len(tx.Entry))

		tx.Get(entry.Resource, "Task/"+*currentTask.Id, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			Action: fhir.AuditEventActionR,
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer:         *request.LocalIdentity,
			AdditionalEntity: []fhir.AuditEventEntity{queryEntity},
		}))
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		results := []*fhir.BundleEntry{}

		for _, idx := range taskEntryIndexes {
			var retTask fhir.Task

			result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &retTask)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to process Task read result: %w", err)
			}
			results = append(results, result)
		}

		// We do not want to notify subscribers for a get
		return results, []any{}, nil
	}, nil
}

// searchTask performs the core functionality of searching for tasks and filtering by authorization
// This can be used by other resources to search for tasks and filter by authorization
func (s *Service) searchTask(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers, principal auth.Principal) (*fhir.Bundle, error) {
	// Verify requester is authenticated
	if principal.Organization.Identifier == nil || len(principal.Organization.Identifier) == 0 {
		return nil, errors.New("not authenticated")
	}

	tasks, bundle, err := handleSearchResource[fhir.Task](ctx, s, "Task", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		// If there are no tasks in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	// It is possible that we have tasks based on different CarePlans. Create distinct list of References to be used for checking participant
	refs := make(map[string]bool)
	for _, task := range tasks {
		for _, bo := range task.BasedOn {
			if bo.Reference == nil || refs[*bo.Reference] {
				continue
			}
			refs[*bo.Reference] = true
		}
	}

	taskRefs := make([]string, 0)
	for ref := range refs {
		for _, task := range tasks {
			isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&task, principal.Organization.Identifier)
			if !(isOwner || isRequester) {
				var carePlan fhir.CarePlan

				if err := s.fhirClient.ReadWithContext(ctx, ref, &carePlan); err != nil {
					continue
				}

				careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
				if err != nil {
					continue
				}

				err = validatePrincipalInCareTeam(principal, careTeam)
				if err != nil {
					continue
				}
			}
			taskRefs = append(taskRefs, "Task/"+*task.Id)
		}
	}
	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"Task"}, taskRefs)

	return &retBundle, nil
}
