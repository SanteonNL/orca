package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdateTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	log.Info().Msgf("Updating Task: %s", request.ResourceId)
	var task fhir.Task
	if err := json.Unmarshal(request.ResourceData, &task); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", task, err)
	}

	// Validate fields on updated Task
	err := coolfhir.ValidateTaskRequiredFields(task)
	if err != nil {
		return nil, fmt.Errorf("invalid Task: %w", err)
	}

	// the Task prior to updates, we need this to validate the state transition
	var taskExisting fhir.Task
	if err := s.fhirClient.Read("Task/"+request.ResourceId, &taskExisting); err != nil {
		return nil, fmt.Errorf("failed to read Task: %w", err)
	}

	// Validate state transition
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&task, principal.Organization.Identifier)
	if !isValidTransition(taskExisting.Status, task.Status, isOwner, isRequester) {
		return nil, errors.New(
			fmt.Sprintf(
				"invalid state transition from %s to %s, owner(%t) requester(%t)",
				taskExisting.Status.String(),
				task.Status.String(),
				isOwner,
				isRequester,
			))
	}

	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		return nil, fmt.Errorf("invalid Task.basedOn: %w", err)
	}

	tx = tx.Append(request.bundleEntryWithResource(task))
	// Update care team
	_, err = careteamservice.Update(s.fhirClient, *carePlanRef, task, tx)
	if err != nil {
		return nil, fmt.Errorf("update CareTeam: %w", err)
	}

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedTask fhir.Task
		result, err := coolfhir.FetchBundleEntry(s.fhirClient, txResult, func(entry fhir.BundleEntry) bool {
			return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, request.ResourcePath)
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

func isValidTransition(from fhir.TaskStatus, to fhir.TaskStatus, isOwner bool, isRequester bool) bool {
	if isOwner == false && isRequester == false {
		return false
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
