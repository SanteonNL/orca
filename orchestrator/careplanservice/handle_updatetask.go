package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func (s *Service) handleUpdateTask(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	taskID := httpRequest.PathValue("id")
	if taskID == "" {
		return errors.New("missing Task ID")
	}
	log.Info().Msgf("Updating Task: %s", taskID)
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
	taskEntry := filterBundleEntry(bundle, func(entry fhir.BundleEntry) bool {
		type Resource struct {
			Type string `json:"resourceType"`
		}
		var res Resource
		if err := json.Unmarshal(entry.Resource, &res); err != nil {
			return false
		}
		return res.Type == "Task"
	})
	if taskEntry == nil {
		// TODO: Might have to do cleanup here?
		return errors.New("could not find Task in FHIR Bundle")
	}
	var headers fhirclient.Headers
	if err := s.fhirClient.Read(*taskEntry.Response.Location, &task, fhirclient.ResponseHeaders(&headers)); err != nil {
		return fmt.Errorf("failed to read created Task from FHIR server: %w", err)
	}
	httpResponse.WriteHeader(http.StatusOK)
	for key, value := range headers.Header {
		httpResponse.Header()[key] = value
	}
	return json.NewEncoder(httpResponse).Encode(task)
}

func (s *Service) updateTaskStatus(taskID string, newStatus fhir.TaskStatus) (*fhir.Bundle, error) {
	if newStatus == fhir.TaskStatusAccepted {

	}
}
