package careplanservice

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

const SCP_TASK_PROFILE = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"

func (s *Service) handleTaskFiller(task map[string]interface{}) error {
	log.Info().Msg("Running Task Filler")

	if !s.isScpTask(task) {
		log.Info().Msg("Task is not an SCP Task - skipping")
		return nil
	}

	if err := s.isValidTask(task); err != nil {
		log.Error().Msgf("Task invalid - skipping: %v", err)
		return fmt.Errorf("task is not valid - skipping: %w", err)
	}

	partOfRef, err := s.partOf(task)
	if err != nil {
		return fmt.Errorf("failed to extract Task.partOf: %w", err)
	}

	// If partOfRef is nil, handle the task as a primary task
	if partOfRef == nil {
		log.Info().Msg("Found a new 'primary' task, checking if more information is needed via a Questionnaire")
		err := s.createSubTask(task)

		if err != nil {
			return fmt.Errorf("failed to process new primary Task: %w", err)
		}
	} else {
		// TODO: Handle sub-task logic, has a QuestionnaireResponse been sent?
		log.Info().Msg("Sub task processing not yet implemented")
	}
	return nil
}

func (s *Service) isScpTask(task map[string]interface{}) bool {
	meta, ok := task["meta"].(map[string]interface{})
	if !ok {
		return false
	}

	profiles, ok := meta["profile"].([]interface{})
	if !ok {
		return false
	}

	for _, profile := range profiles {
		if profileStr, ok := profile.(string); ok && profileStr == SCP_TASK_PROFILE {
			return true
		}
	}

	return false
}

func (s *Service) isValidTask(task map[string]interface{}) error {

	requiredFields := []string{"requester", "owner", "id", "basedOn"}

	for _, field := range requiredFields {
		if task[field] == nil {
			return fmt.Errorf("task must have a %s", field)
		}
	}

	return nil
}

func (s *Service) createSubTask(task map[string]interface{}) error {
	questionnaire := s.getHardCodedHomeMonitoringQuestionnaire()

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := s.getSubTask(task, questionnaireRef)
	subtaskRef := "urn:uuid:" + subtask["id"].(string)

	tx := coolfhir.Transaction().
		Create(questionnaire, coolfhir.WithFullUrl(questionnaireRef)).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	bundle := tx.Bundle()

	resultBundle, err := coolfhir.ExecuteTransaction(s.fhirClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Info().Msgf("Successfully created a subtask - tsx contained %d resources", resultBundle.Total)

	return nil
}

// getSubTask creates a new subtask in map[string]interface{} format
func (s *Service) getSubTask(task map[string]interface{}, questionnaireRef string) map[string]interface{} {

	partOf := []map[string]interface{}{
		{
			"reference": "Task/" + task["id"].(string),
		},
	}

	subtask := map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Task",
		"status":       "ready",
		"meta": map[string]interface{}{
			"profile": []string{
				SCP_TASK_PROFILE,
			},
		},
		"basedOn":   task["basedOn"],
		"partOf":    partOf,
		"focus":     task["focus"],
		"for":       task["for"],
		"owner":     task["requester"], //reversed
		"requester": task["owner"],     //reversed
		"input": []map[string]interface{}{
			{
				"type": map[string]interface{}{
					"coding": []map[string]interface{}{
						{
							"system":  "http://terminology.hl7.org/CodeSystem/task-input-type",
							"code":    "Reference",
							"display": "Reference",
						},
					},
				},
				"valueReference": map[string]interface{}{
					"reference": questionnaireRef,
				},
			},
		},
	}

	return subtask
}

func (s *Service) partOf(task map[string]interface{}) (*string, error) {
	partOf, exists := task["partOf"]
	if !exists {
		return nil, nil // Optional reference, simply return nil if not set
	}

	var taskPartOf []fhir.Reference
	if err := convertInto(partOf, &taskPartOf); err != nil {
		return nil, fmt.Errorf("failed to convert Task.partOf: %w", err)
	}

	if len(taskPartOf) != 1 {
		return nil, errors.New("Task.partOf must have exactly one reference")
	} else if taskPartOf[0].Reference == nil || !strings.HasPrefix(*taskPartOf[0].Reference, "Task/") {
		return nil, errors.New("Task.partOf must contain a relative reference to a Task")
	}

	return taskPartOf[0].Reference, nil
}

func (s *Service) getHardCodedHomeMonitoringQuestionnaire() map[string]interface{} {
	return map[string]interface{}{
		"id":           "cps-questionnaire-telemonitoring-enrollment-criteria",
		"resourceType": "Questionnaire",
		"meta": map[string]interface{}{
			"source": "http://decor.nictiz.nl/fhir/4.0/sansa-",
			"tag": []map[string]interface{}{
				{
					"system": "http://hl7.org/fhir/FHIR-version",
					"code":   "4.0.1",
				},
			},
		},
		"language": "nl-NL",
		"url":      "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017",
		"identifier": []map[string]interface{}{
			{
				"system": "urn:ietf:rfc:3986",
				"value":  "urn:oid:2.16.840.1.113883.2.4.3.11.60.909.26.34-1",
			},
		},
		"name":         "Telemonitoring - enrollment criteria",
		"title":        "Telemonitoring - enrollment criteria",
		"status":       "active",
		"experimental": false,
		"date":         "2024-09-02T13:40:17Z",
		"publisher":    "Medical Service Centre",
		"effectivePeriod": map[string]interface{}{
			"start": "2024-09-02T13:40:17Z",
		},
		"item": []map[string]interface{}{
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2208",
				"text":     "Patient heeft smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2209",
				"text":     "Patient of mantelzorger leest e-mail op smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2210",
				"text":     "Patient of mantelzorger kan apps installeren op smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2211",
				"text":     "Patient of mantelzorger is Nederlandse taal machtig",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2212",
				"text":     "Patient beschikt over een weegschaal of bloeddrukmeter (of gaat deze aanschaffen)",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
		},
	}
}
