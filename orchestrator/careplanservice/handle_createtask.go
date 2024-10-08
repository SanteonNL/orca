package careplanservice

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateTask(httpRequest *http.Request, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	log.Info().Msg("Creating Task")
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	var task fhir.Task
	if err := s.readRequest(httpRequest, &task); err != nil {
		return nil, fmt.Errorf("invalid Task: %w", err)
	}

	switch task.Status {
	case fhir.TaskStatusRequested:
	case fhir.TaskStatusReady:
	default:
		return nil, errors.New(fmt.Sprintf("cannot create Task with status %s, must be %s or %s", task.Status, fhir.TaskStatusRequested.String(), fhir.TaskStatusReady.String()))
	}
	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		//FIXME: This logic changed, create a new CarePlan when Task.basedOn is not set
		return nil, fmt.Errorf("invalid Task.basedOn: %w", err)
	}
	// TODO: Manage time-outs properly
	var carePlan fhir.CarePlan
	if err := s.fhirClient.Read(*carePlanRef, &carePlan); err != nil {
		return nil, fmt.Errorf("failed to read CarePlan: %w", err)
	}
	// Add Task to CarePlan.activities
	err = s.newTaskInCarePlan(tx, task, &carePlan)
	if err != nil {
		return nil, fmt.Errorf("failed to create Task: %w", err)
	}
	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdTask fhir.Task
		result, err := coolfhir.FetchBundleEntry(s.fhirClient, txResult, func(entry fhir.BundleEntry) bool {
			return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/")
		}, &createdTask)
		if err != nil {
			return nil, nil, err
		}
		var notifications = []any{&createdTask}
		// If CareTeam was updated, notify about CareTeam
		var updatedCareTeam fhir.CareTeam
		if err := coolfhir.ResourceInBundle(txResult, coolfhir.EntryIsOfType("CareTeam"), &updatedCareTeam); err == nil {
			notifications = append(notifications, &updatedCareTeam)
		}

		// TODO: this really shouldn't be here. Maybe move to CPC and call it in-process (e.g. goroutine?)
		// TODO: This should be done by the CPC after the notification is received
		if err = s.handleTaskFillerCreate(&createdTask); err != nil {
			return nil, nil, fmt.Errorf("failed to handle TaskFillerCreate: %w", err)
		}
		return result, []any{&createdTask}, nil
	}, nil
}

// newTaskInCarePlan creates a new Task and references the Task from the CarePlan.activities.
func (s *Service) newTaskInCarePlan(tx *coolfhir.TransactionBuilder, task fhir.Task, carePlan *fhir.CarePlan) error {
	taskRef := "urn:uuid:" + uuid.NewString()
	carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
		Reference: &fhir.Reference{
			Reference: to.Ptr(taskRef),
			Type:      to.Ptr("Task"),
		},
	})

	// TODO: Only if not updated
	tx.Create(task, coolfhir.WithFullUrl(taskRef)).
		Update(*carePlan, "CarePlan/"+*carePlan.Id)

	if _, err := careteamservice.Update(s.fhirClient, *carePlan.Id, task, tx); err != nil {
		return fmt.Errorf("failed to update CarePlan: %w", err)
	}
	return nil
}

// basedOn returns the CarePlan reference the Task is based on, e.g. CarePlan/123.
func basedOn(task fhir.Task) (*string, error) {
	if len(task.BasedOn) != 1 {
		return nil, errors.New("Task.basedOn must have exactly one reference")
	} else if task.BasedOn[0].Reference == nil || !strings.HasPrefix(*task.BasedOn[0].Reference, "CarePlan/") {
		return nil, errors.New("Task.basedOn must contain a relative reference to a CarePlan")
	}
	return task.BasedOn[0].Reference, nil
}
