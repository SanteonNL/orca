package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func (s *Service) handleCreateTask(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	log.Info().Msg("Creating Task")
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	task := make(map[string]interface{})
	if err := s.readRequest(httpRequest, &task); err != nil {
		return fmt.Errorf("invalid Task: %w", err)
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
	// Find right result to return
	taskEntry := coolfhir.FirstBundleEntry(bundle, coolfhir.EntryIsOfType("Task"))
	if taskEntry == nil {
		// TODO: Might have to do cleanup here?
		return errors.New("could not find Task in FHIR Bundle")
	}
	var headers fhirclient.Headers
	if err := s.fhirClient.Read(*taskEntry.Response.Location, &task, fhirclient.ResponseHeaders(&headers)); err != nil {
		return fmt.Errorf("failed to read created Task from FHIR server: %w", err)
	}
	for key, value := range headers.Header {
		httpResponse.Header()[key] = value
	}
	httpResponse.WriteHeader(http.StatusCreated)
	return json.NewEncoder(httpResponse).Encode(task)
}

// newTaskInCarePlan creates a new Task and references the Task from the CarePlan.activities.
func (s *Service) newTaskInCarePlan(task map[string]interface{}, carePlan *fhir.CarePlan) (*fhir.Bundle, error) {
	taskFullURL := "urn:uuid:" + uuid.NewString()
	carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
		Reference: &fhir.Reference{
			Reference: to.Ptr(taskFullURL),
			Type:      to.Ptr("Task"),
		},
	})

	carePlanData, _ := json.Marshal(*carePlan)
	// TODO: Only if not updated
	taskData, _ := json.Marshal(task)
	bundle := fhir.Bundle{
		Type: fhir.BundleTypeTransaction,
		Entry: []fhir.BundleEntry{
			// Create Task
			{
				FullUrl:  to.Ptr(taskFullURL),
				Resource: taskData,
				Request: &fhir.BundleEntryRequest{
					Method: fhir.HTTPVerbPOST,
					Url:    "Task",
				},
			},
			// Update CarePlan
			{
				Resource: carePlanData,
				Request: &fhir.BundleEntryRequest{
					Method: fhir.HTTPVerbPUT,
					Url:    "CarePlan/" + *carePlan.Id,
				},
			},
		},
	}
	if err := s.fhirClient.Create(bundle, &bundle, fhirclient.AtPath("/")); err != nil {
		return nil, fmt.Errorf("failed to create Task and update CarePlan: %w", err)
	}
	return &bundle, nil
}

// basedOn returns the CarePlan reference the Task is based on, e.g. CarePlan/123.
func basedOn(task map[string]interface{}) (*string, error) {
	var taskBasedOn []fhir.Reference
	if err := convertInto(task["basedOn"], &taskBasedOn); err != nil {
		return nil, fmt.Errorf("failed to convert Task.basedOn: %w", err)
	} else if len(taskBasedOn) != 1 {
		return nil, errors.New("Task.basedOn must have exactly one reference")
	} else if taskBasedOn[0].Type == nil || *taskBasedOn[0].Type != "CarePlan" || taskBasedOn[0].Reference == nil {
		return nil, errors.New("Task.basedOn must reference a CarePlan")
	}
	return taskBasedOn[0].Reference, nil
}
