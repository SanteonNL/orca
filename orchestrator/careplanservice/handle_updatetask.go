package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdateTask(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	taskID := httpRequest.PathValue("id")
	if taskID == "" {
		return errors.New("missing Task ID")
	}
	log.Info().Msgf("Updating Task: %s", taskID)
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	var task coolfhir.Task
	if err := s.readRequest(httpRequest, &task); err != nil {
		return fmt.Errorf("invalid Task: %w", err)
	}

	// the Task prior to updates, we need this to validate the state transition
	var taskExisting coolfhir.Task
	if err := s.fhirClient.Read("Task/"+taskID, &taskExisting); err != nil {
		return fmt.Errorf("failed to read Task: %w", err)
	}

	// Validate state transition
	taskFHIR, err := task.ToFHIR()
	if err != nil {
		return err
	}
	taskExistingFHIR, err := taskExisting.ToFHIR()
	if err != nil {
		return err
	}

	principal, err := auth.PrincipalFromContext(httpRequest.Context())
	if err != nil {
		return err
	}
	var isOwner bool
	if taskFHIR.Owner != nil {
		for _, identifier := range principal.Organization.Identifier {
			if coolfhir.LogicalReferenceEquals(*taskFHIR.Owner, fhir.Reference{Identifier: &identifier}) {
				isOwner = true
				break
			}
		}
	}
	var isRequester bool
	if taskFHIR.Requester != nil {
		for _, identifier := range principal.Organization.Identifier {
			if coolfhir.LogicalReferenceEquals(*taskFHIR.Requester, fhir.Reference{Identifier: &identifier}) {
				isRequester = true
				break
			}
		}
	}
	if !isValidTransition(taskExistingFHIR.Status, taskFHIR.Status, isOwner, isRequester) {
		return errors.New(
			fmt.Sprintf(
				"invalid state transition from %s to %s, owner(%t) requester(%t)",
				taskExistingFHIR.Status.String(),
				taskFHIR.Status.String(),
				isOwner,
				isRequester,
			))
	}

	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		return fmt.Errorf("invalid Task.basedOn: %w", err)
	}

	tx := coolfhir.Transaction()
	tx = tx.Update(task, "Task/"+taskID)
	r4Task, err := task.ToFHIR()
	if err != nil {
		return err
	}
	// Update care team
	careTeamUpdated, err := careteamservice.Update(s.fhirClient, *carePlanRef, *r4Task, tx)
	if err != nil {
		return fmt.Errorf("update CareTeam: %w", err)
	}

	// Perform update
	if _, err := coolfhir.ExecuteTransactionAndRespondWithEntry(s.fhirClient, tx.Bundle(), func(entry fhir.BundleEntry) bool {
		return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/"+taskID)
	}, httpResponse); err != nil {
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original Task that was sent.
			httpResponse.WriteHeader(http.StatusOK)
			return json.NewEncoder(httpResponse).Encode(task)
		}
		return fmt.Errorf("failed to update Task (CareTeam updated=%v): %w", careTeamUpdated, err)
	}
	return nil
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
