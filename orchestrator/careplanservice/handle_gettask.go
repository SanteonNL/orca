package careplanservice

import (
	"context"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetTask fetches the requested Task and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the task)
// if the requester is valid, return the Task, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetTask(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Task, error) {
	// fetch Task + CareTeam, validate requester is participant of CareTeam
	var task fhir.Task
	err := s.fhirClient.ReadWithContext(ctx, "Task/"+id, &task, fhirclient.ResponseHeaders(headers))
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

	return &task, nil
}

// handleSearchTask does a search for Task based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchTask(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	_, bundle, err := handleSearchResource[fhir.Task](ctx, s, "Task", queryParams, headers)
	if err != nil {
		return nil, err
	}

	return bundle, nil
}
