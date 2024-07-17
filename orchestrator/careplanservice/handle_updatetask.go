package careplanservice

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"strings"
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

	bundle := new(fhir.Bundle)
	if err := s.fhirClient.Read("CarePlan",
		bundle,
		fhirclient.QueryParam("_id", strings.TrimPrefix(*carePlanRef, "CarePlan/")),
		fhirclient.QueryParam("_include", "CarePlan:care-team"),
		fhirclient.QueryParam("_include", "CarePlan:activity-reference")); err != nil {
		return fmt.Errorf("unable to resolve CarePlan and related resources: %w", err)
	}
	var carePlan fhir.CarePlan
	if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryHasID(*carePlanRef), &carePlan); err != nil {
		return fmt.Errorf("CarePlan not in Bundle (ref=%s): %w", *carePlanRef, err)
	}
	if len(carePlan.CareTeam) != 1 || carePlan.CareTeam[0].Reference == nil {
		return errors.New("expected CarePlan to have exactly one CareTeam, with a reference")
	}
	var careTeam fhir.CareTeam
	if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryHasID(*carePlan.CareTeam[0].Reference), &careTeam); err != nil {
		return fmt.Errorf("CareTeam not in Bundle (ref=%s): %w", *carePlan.CareTeam[0].Reference, err)
	}
	var activities []fhir.Task
	if err := coolfhir.ResourcesInBundle(bundle, coolfhir.EntryIsOfType("Task"), &activities); err != nil {
		return err
	}
	return nil
}

//func (s *Service) updateTaskStatus(taskID string, newStatus fhir.TaskStatus) (*fhir.Bundle, error) {
//	if newStatus == fhir.TaskStatusAccepted {
//
//	}
//}
