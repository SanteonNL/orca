package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"strings"
)

func (s *Service) handleUpdateTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Info().Msgf("Updating Task: %s", request.RequestUrl)
	var task fhir.Task
	if err := json.Unmarshal(request.ResourceData, &task); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", task, err)
	}

	// Validate fields on updated Task
	err := coolfhir.ValidateTaskRequiredFields(task)
	if err != nil {
		return nil, fmt.Errorf("invalid Task: %w", err)
	}

	var taskExisting fhir.Task
	exists := true
	if request.ResourceId == "" {
		// No ID, should be query parameters leading to the Task to update
		if len(request.RequestUrl.Query()) == 0 {
			return nil, errors.New("missing Task ID or query parameters for selecting the Task to update")
		}
		var opts []fhirclient.Option
		for k, v := range request.RequestUrl.Query() {
			opts = append(opts, fhirclient.QueryParam(k, v[0]))
		}
		var resultBundle fhir.Bundle
		if err = s.fhirClient.Read("Task", &resultBundle, opts...); err != nil {
			return nil, fmt.Errorf("failed to search for Task to update: %w", err)
		}
		if len(resultBundle.Entry) == 0 {
			exists = false
		} else if len(resultBundle.Entry) > 1 {
			return nil, errors.New("multiple Tasks found to update, expected 1")
		} else {
			if err = coolfhir.ResourceInBundle(&resultBundle, coolfhir.EntryIsOfType("Task"), &taskExisting); err != nil {
				return nil, fmt.Errorf("failed to read Task from search result: %w", err)
			}
		}
		if task.Id != nil && *taskExisting.Id != *task.Id {
			return nil, coolfhir.BadRequestError("ID in request URL does not match ID in resource")
		}
	} else {
		if (task.Id != nil && request.ResourceId != "") && request.ResourceId != *task.Id {
			return nil, coolfhir.BadRequestError("ID in request URL does not match ID in resource")
		}
		err = s.fhirClient.Read("Task/"+request.ResourceId, &taskExisting)
		// TODO: If the resource was identified by a concrete ID, and was intended as upsert (create-if-not-exists), this doesn't work yet.
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read Task: %w", err)
	}
	if !exists {
		// Doesn't exist, create it (upsert)
		return s.handleCreateTask(ctx, request, tx)
	}

	// Prior to the Task update, we need this to validate the state transition
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&taskExisting, principal.Organization.Identifier)
	isScpSubTask := coolfhir.IsScpSubTask(&task)
	if !isValidTransition(taskExisting.Status, task.Status, isOwner, isRequester, isScpSubTask) {
		return nil, errors.New(
			fmt.Sprintf(
				"invalid state transition from %s to %s, owner(%t) requester(%t) scpSubtask(%t)",
				taskExisting.Status.String(),
				task.Status.String(),
				isOwner,
				isRequester,
				isScpSubTask,
			))
	}

	// Check fields that aren't allowed to be changed: owner, requester, basedOn, partOf, for
	if !deep.Equal(task.Requester, taskExisting.Requester) {
		return nil, errors.New("Task.requester cannot be changed")
	}
	if !deep.Equal(task.Owner, taskExisting.Owner) {
		return nil, errors.New("Task.owner cannot be changed")
	}
	if !deep.Equal(task.BasedOn, taskExisting.BasedOn) {
		return nil, errors.New("Task.basedOn cannot be changed")
	}
	if !deep.Equal(task.PartOf, taskExisting.PartOf) {
		return nil, errors.New("Task.partOf cannot be changed")
	}
	if !deep.Equal(task.For, taskExisting.For) {
		return nil, errors.New("Task.for cannot be changed")
	}

	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		return nil, fmt.Errorf("invalid Task.basedOn: %w", err)
	}
	carePlanId := strings.TrimPrefix(*carePlanRef, "CarePlan/")

	taskBundleEntry := request.bundleEntryWithResource(task)
	tx = tx.AppendEntry(taskBundleEntry)
	idx := len(tx.Entry) - 1
	// Update care team
	_, err = careteamservice.Update(s.fhirClient, carePlanId, task, tx)
	if err != nil {
		return nil, fmt.Errorf("update CareTeam: %w", err)
	}

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedTask fhir.Task
		result, err := coolfhir.FetchBundleEntry(s.fhirClient, s.fhirURL, &taskBundleEntry, txResult, func(currIdx int, entry fhir.BundleEntry) bool {
			return currIdx == idx
		}, &updatedTask)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original Task that was sent.
			updatedTask = task
		} else if err != nil {
			// Other error
			return nil, nil, err
		}
		var notifications = []any{&updatedTask}
		// If CareTeam was updated, notify about CareTeam
		var updatedCareTeam fhir.CareTeam
		if err := coolfhir.ResourceInBundle(txResult, coolfhir.EntryIsOfType("CareTeam"), &updatedCareTeam); err == nil {
			notifications = append(notifications, &updatedCareTeam)
		}
		return result, notifications, nil
	}, nil
}

func isValidTransition(from fhir.TaskStatus, to fhir.TaskStatus, isOwner bool, isRequester bool, isScpSubtask bool) bool {
	if isOwner == false && isRequester == false {
		return false
	}

	if isScpSubtask {
		return isOwner && from == fhir.TaskStatusReady && (to == fhir.TaskStatusCompleted || to == fhir.TaskStatusFailed)
	}

	// Transitions valid for owner only
	if isOwner {
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusReceived {
			return true
		}
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusAccepted {
			return true
		}
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusRejected {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusAccepted {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusRejected {
			return true
		}
		if from == fhir.TaskStatusAccepted && to == fhir.TaskStatusInProgress {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusCompleted {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusFailed {
			return true
		}
		if from == fhir.TaskStatusReady && to == fhir.TaskStatusCompleted {
			return true
		}
		if from == fhir.TaskStatusReady && to == fhir.TaskStatusFailed {
			return true
		}
	}
	// Transitions valid for owner or requester
	if isOwner || isRequester {
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusAccepted && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusOnHold {
			return true
		}
		if from == fhir.TaskStatusOnHold && to == fhir.TaskStatusInProgress {
			return true
		}
	}
	return false
}
