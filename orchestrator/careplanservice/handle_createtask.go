package careplanservice

import (
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateTask(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	log.Info().Msg("Creating Task")
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	task := make(map[string]interface{})
	if err := s.readRequest(httpRequest, &task); err != nil {
		return fmt.Errorf("invalid Task: %w", err)
	}
	switch task["status"] {
	case fhir.TaskStatusDraft.String():
		task["status"] = fhir.TaskStatusRequested.String()
	case fhir.TaskStatusRequested.String():
	case fhir.TaskStatusReady.String():
	default:
		return errors.New(fmt.Sprintf("cannot create task with status %s, must be %s or %s", task["status"], fhir.TaskStatusRequested.String(), fhir.TaskStatusReady.String()))
	}
	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		return fmt.Errorf("invalid Task.basedOn: %w", err)
	}
	// TODO: Manage time-outs properly
	var carePlan fhir.CarePlan
	if err := s.fhirClient.Read(*carePlanRef, &carePlan); err != nil {
		return fmt.Errorf("failed to read CarePlan: %w", err)
	}
	// Add Task to CarePlan.activities
	bundle, err := s.newTaskInCarePlan(task, &carePlan)
	if err != nil {
		return fmt.Errorf("failed to create Task: %w", err)
	}
	return coolfhir.ExecuteTransactionAndRespondWithEntry(s.fhirClient, *bundle, func(entry fhir.BundleEntry) bool {
		return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/")
	}, httpResponse)
}

// newTaskInCarePlan creates a new Task and references the Task from the CarePlan.activities.
func (s *Service) newTaskInCarePlan(task coolfhir.Task, carePlan *fhir.CarePlan) (*fhir.Bundle, error) {
	taskRef := "urn:uuid:" + uuid.NewString()
	carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
		Reference: &fhir.Reference{
			Reference: to.Ptr(taskRef),
			Type:      to.Ptr("Task"),
		},
	})

	// TODO: Only if not updated
	tx := coolfhir.Transaction().
		Create(task, coolfhir.WithFullUrl(taskRef)).
		Update(*carePlan, "CarePlan/"+*carePlan.Id)

	r4Task, err := task.ToFHIR()
	if err != nil {
		return nil, err
	}
	if _, err = careteamservice.Update(s.fhirClient, *carePlan.Id, *r4Task, tx); err != nil {
		return nil, fmt.Errorf("failed to update CarePlan: %w", err)
	}
	result := tx.Bundle()
	return &result, nil
}

// basedOn returns the CarePlan reference the Task is based on, e.g. CarePlan/123.
func basedOn(task map[string]interface{}) (*string, error) {
	var taskBasedOn []fhir.Reference
	if err := convertInto(task["basedOn"], &taskBasedOn); err != nil {
		return nil, fmt.Errorf("failed to convert Task.basedOn: %w", err)
	} else if len(taskBasedOn) != 1 {
		return nil, errors.New("Task.basedOn must have exactly one reference")
	} else if taskBasedOn[0].Reference == nil || !strings.HasPrefix(*taskBasedOn[0].Reference, "CarePlan/") {
		return nil, errors.New("Task.basedOn must contain a relative reference to a CarePlan")
	}
	return taskBasedOn[0].Reference, nil
}
