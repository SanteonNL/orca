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
)

func ReadTaskAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Task] {
	return AnyMatchPolicy[fhir.Task]{
		Policies: []Policy[fhir.Task]{
			TaskOwnerOrRequesterPolicy[fhir.Task]{},
			CareTeamMemberPolicy[fhir.Task]{
				fhirClient: fhirClient,
				carePlanRefFunc: func(resource fhir.Task) ([]string, error) {
					var refs []string
					for _, reference := range resource.BasedOn {
						if reference.Reference != nil {
							refs = append(refs, *reference.Reference)
						}
					}
					return refs, nil
				},
			},
		},
	}
}

// handleReadTask fetches the requested Task and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the task)
// if the requester is valid, return the Task, else return an error
func (s *Service) handleReadTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
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

	taskRaw, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: taskRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        task.Id,
		Type:      to.Ptr("Task"),
		Reference: to.Ptr("Task/" + *task.Id),
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
