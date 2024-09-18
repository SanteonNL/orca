package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

const SCP_TASK_PROFILE = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"

func (s *Service) handleTaskFillerUpdate(task map[string]interface{}) error {

	log.Info().Msg("Running handleTaskFillerUpdate")

	if !s.isScpTask(task) {
		log.Info().Msg("Task is not an SCP Task - skipping")
		return nil
	}

	if task["status"] != "completed" {
		log.Info().Msg("Task.status is not completed - skipping")
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

	if partOfRef == nil {
		return errors.New("handleTaskFillerUpdate got a subtask without a partOf set")
	}

	log.Info().Msg("SubTask.status is completed - processing")

	// fetch the questionnaire from the task.input
	var questionnaire map[string]interface{}
	input, ok := task["input"].([]interface{})
	if !ok {
		return errors.New("task.input is not a valid array")
	}

	var questionnaireRefs []string
	for _, item := range input {
		if inputMap, ok := item.(map[string]interface{}); ok {
			if valueRef, ok := inputMap["valueReference"].(map[string]interface{}); ok {
				if ref, ok := valueRef["reference"].(string); ok && strings.HasPrefix(ref, "Questionnaire/") {
					questionnaireRefs = append(questionnaireRefs, ref)
				}
			}
		}
	}

	if len(questionnaireRefs) != 1 {
		return fmt.Errorf("expected exactly 1 Questionnaire reference, found %d", len(questionnaireRefs))
	}

	questionnaire = make(map[string]interface{})
	err = s.fetchQuestionnaireByID(questionnaireRefs[0], questionnaire)
	if err != nil {
		return fmt.Errorf("failed to fetch questionnaire: %w", err)
	}

	questionnaireJSON, err := json.MarshalIndent(questionnaire, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal questionnaire to JSON: %w", err)
	}
	log.Info().Msgf("Questionnaire: %s", string(questionnaireJSON))

	// if questionnaire {
	// 	return errors.New("no valid questionnaire reference found in task.input")
	// }

	if questionnaire["url"] == "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017" {

		//TODO: Conditional create on not existing
		return s.createSubTaskPII(task)
	}

	if questionnaire["url"] == "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017" {

		log.Info().Msg("SubTask with PII is completed - marking primary Task (partOf value) as accepted")

		// Fetch task from Task.partOf
		partOfRefs, ok := task["partOf"].([]interface{})
		if !ok || len(partOfRefs) == 0 {
			return errors.New("task.partOf is not a valid array or is empty")
		}

		partOfRef, ok := partOfRefs[0].(map[string]interface{})
		if !ok {
			return errors.New("task.partOf[0] is not a valid reference")
		}

		partOfTaskRef, ok := partOfRef["reference"].(string)
		if !ok || !strings.HasPrefix(partOfTaskRef, "Task/") {
			return errors.New("task.partOf[0].reference is not a valid Task reference")
		}

		var partOfTask map[string]interface{}
		err := s.fhirClient.Read(partOfTaskRef, &partOfTask)
		if err != nil {
			return fmt.Errorf("failed to fetch partOf Task: %w", err)
		}

		// Change status to completed
		partOfTask["status"] = "accepted"

		// Update the task in the FHIR server
		err = s.fhirClient.Update(partOfTaskRef, partOfTask, partOfTask)
		if err != nil {
			return fmt.Errorf("failed to update partOf %s status: %w", partOfRef, err)
		}

		log.Info().Msgf("Successfully marked %s as completed", partOfTaskRef)
	}

	return nil
}

func (s *Service) fetchQuestionnaireByID(ref string, questionnaire map[string]interface{}) error {

	log.Info().Msg("Fetching Questionnaire by ID")

	err := s.fhirClient.Read(ref, &questionnaire)

	if err != nil {
		return fmt.Errorf("failed to fetch Questionnaire: %w", err)
	}

	return nil
}

func (s *Service) handleTaskFillerCreate(task map[string]interface{}) error {
	log.Info().Msg("Running handleTaskFillerCreate")

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
		err := s.createSubTaskEnrollmentCriteria(task)

		if err != nil {
			return fmt.Errorf("failed to process new primary Task: %w", err)
		}
	} else {
		return s.handleSubTaskCreate(task)

	}
	return nil
}

func (s *Service) handleSubTaskCreate(task map[string]interface{}) error {
	log.Info().Msgf("SubTask for Task/%s create handling not implemented - skipping", task["id"])
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

func (s *Service) createSubTaskEnrollmentCriteria(task map[string]interface{}) error {
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

func (s *Service) createSubTaskPII(task map[string]interface{}) error {

	log.Info().Msg("Creating a new PII subtask")

	questionnaire := s.getHardCodedHomeMonitoringPIIQuestionnaire()

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := s.getPIISubTask(task, questionnaireRef)
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

// Generates the PII subtask - provide the initial enrollment subtask as argument
func (s *Service) getPIISubTask(task map[string]interface{}, questionnaireRef string) map[string]interface{} {

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
		"partOf":    task["partOf"],
		"focus":     task["focus"],
		"for":       task["for"],
		"owner":     task["owner"],
		"requester": task["requester"],
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
