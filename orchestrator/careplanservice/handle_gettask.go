package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

// handleGetTask fetches the requested Task and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the task)
// if the requester is valid, return the Task, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetTask(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Task, error) {
	// fetch Task + CareTeam, validate requester is participant of CareTeam
	var task fhir.Task
	err := s.fhirClient.Read("Task/"+id, &task, fhirclient.ResponseHeaders(headers))
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

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Check if the requester is either the task Owner or Requester, if not, they must be a member of the CareTeam
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&task, principal.Organization.Identifier)
	if !(isOwner || isRequester) {
		_, careTeams, _, err := s.getCarePlanAndCareTeams(*task.BasedOn[0].Reference)
		if err != nil {
			return nil, err
		}

		err = validatePrincipalInCareTeams(ctx, careTeams)
		if err != nil {
			return nil, err
		}
	}

	return &task, nil
}

// handleSearchTask does a search for Task based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchTask(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	params := []fhirclient.Option{}
	for k, v := range queryParams {
		params = append(params, fhirclient.QueryParam(k, v[0]))
	}

	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read("Task", &bundle, params...)
	if err != nil {
		return nil, err
	}

	var tasks []fhir.Task
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Task"), &tasks)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		// If there are no tasks in the bundle there is no point in doing validation, return empty bundle to user
		return &bundle, nil
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

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	for ref, _ := range refs {
		for _, task := range tasks {
			isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&task, principal.Organization.Identifier)
			if !(isOwner || isRequester) {
				_, careTeams, _, err := s.getCarePlanAndCareTeams(ref)
				if err != nil {
					return nil, err
				}

				err = validatePrincipalInCareTeams(ctx, careTeams)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	return &bundle, nil
}
