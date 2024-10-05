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

func (s *Service) handleCreateTask(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	log.Info().Msg("Creating Task")
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	var task fhir.Task
	if err := s.readRequest(httpRequest, &task); err != nil {
		return fmt.Errorf("invalid Task: %w", err)
	}

	switch task.Status {
	case fhir.TaskStatusRequested:
	case fhir.TaskStatusReady:
	default:
		return errors.New(fmt.Sprintf("cannot create Task with status %s, must be %s or %s", task.Status, fhir.TaskStatusRequested.String(), fhir.TaskStatusReady.String()))
	}
	// Resolve the CarePlan
	carePlanRef, err := basedOn(task)
	if err != nil {
		//FIXME: This logic changed, create a new CarePlan when Task.basedOn is not set
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

	var createdTask fhir.Task
	err = coolfhir.ExecuteTransactionAndRespondWithEntry(s.fhirClient, *bundle, func(entry fhir.BundleEntry) bool {
		return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/")
	}, httpResponse, &createdTask)
	if err != nil {
		return err
	}

	s.notifySubscribers(httpRequest.Context(), &createdTask)
	// If CareTeam was updated, notify about CareTeam
	var updatedCareTeam fhir.CareTeam
	if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryIsOfType("CareTeam"), &updatedCareTeam); err == nil {
		s.notifySubscribers(httpRequest.Context(), &updatedCareTeam)
	}

	return s.handleTaskFillerCreate(&createdTask) //TODO: This should be done by the CPC after the notification is received
}

// newTaskInCarePlan creates a new Task and references the Task from the CarePlan.activities.
func (s *Service) newTaskInCarePlan(task fhir.Task, carePlan *fhir.CarePlan) (*fhir.Bundle, error) {
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

	if _, err := careteamservice.Update(s.fhirClient, *carePlan.Id, task, tx); err != nil {
		return nil, fmt.Errorf("failed to update CarePlan: %w", err)
	}
	result := tx.Bundle()
	return &result, nil
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
