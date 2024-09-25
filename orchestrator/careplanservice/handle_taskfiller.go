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

func (s *Service) handleTaskFillerUpdate(task *fhir.Task) error {

	log.Info().Msg("Running handleTaskFillerUpdate")

	if !s.isScpTask(task) {
		log.Info().Msg("Task is not an SCP Task - skipping")
		return nil
	}

	if task.Status != fhir.TaskStatusCompleted {
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
	var questionnaire fhir.Questionnaire
	var questionnaireRefs []string
	for _, item := range task.Input {
		if ref := item.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "Questionnaire/") {
			questionnaireRefs = append(questionnaireRefs, *ref.Reference)
		}
	}

	if len(questionnaireRefs) != 1 {
		return fmt.Errorf("expected exactly 1 Questionnaire reference, found %d", len(questionnaireRefs))
	}

	err = s.fetchQuestionnaireByID(questionnaireRefs[0], &questionnaire)
	if err != nil {
		return fmt.Errorf("failed to fetch questionnaire: %w", err)
	}

	questionnaireJSON, err := json.MarshalIndent(questionnaire, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal questionnaire to JSON: %w", err)
	}
	log.Info().Msgf("Questionnaire: %s", string(questionnaireJSON))

	if *questionnaire.Url == "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017" {
		return s.createSubTaskPII(task)
	}

	if *questionnaire.Url == "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017" {
		log.Info().Msg("SubTask with PII is completed - marking primary Task (partOf value) as accepted")

		// Fetch task from Task.partOf
		if len(task.PartOf) == 0 {
			return errors.New("task.partOf is empty")
		}

		partOfTaskRef := task.PartOf[0].Reference
		if partOfTaskRef == nil || !strings.HasPrefix(*partOfTaskRef, "Task/") {
			return errors.New("task.partOf[0].reference is not a valid Task reference")
		}

		var partOfTask fhir.Task
		err := s.fhirClient.Read(*partOfTaskRef, &partOfTask)
		if err != nil {
			return fmt.Errorf("failed to fetch partOf Task: %w", err)
		}

		// Change status to accepted
		partOfTask.Status = fhir.TaskStatusAccepted

		// Update the task in the FHIR server
		err = s.fhirClient.Update(*partOfTaskRef, &partOfTask, &partOfTask)
		if err != nil {
			return fmt.Errorf("failed to update partOf %s status: %w", *partOfTaskRef, err)
		}

		log.Info().Msgf("Successfully marked %s as completed", *partOfTaskRef)
	}

	return nil
}

func (s *Service) fetchQuestionnaireByID(ref string, questionnaire *fhir.Questionnaire) error {
	log.Info().Msg("Fetching Questionnaire by ID")

	err := s.fhirClient.Read(ref, &questionnaire)
	if err != nil {
		return fmt.Errorf("failed to fetch Questionnaire: %w", err)
	}

	return nil
}

func (s *Service) handleTaskFillerCreate(task *fhir.Task) error {
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

func (s *Service) handleSubTaskCreate(task *fhir.Task) error {
	log.Info().Msgf("SubTask for Task/%s create handling not implemented - skipping", *task.Id)
	return nil
}

func (s *Service) isScpTask(task *fhir.Task) bool {
	if task.Meta == nil {
		return false
	}

	for _, profile := range task.Meta.Profile {
		if profile == SCP_TASK_PROFILE {
			return true
		}
	}

	return false
}

func (s *Service) isValidTask(task *fhir.Task) error {
	var errs []string

	if task.Id == nil {
		errs = append(errs, "Task.id is required but not provided")
	}
	if task.Requester == nil {
		errs = append(errs, "Task.requester is required but not provided")
	}
	if task.Owner == nil {
		errs = append(errs, "Task.owner is required but not provided")
	}
	if task.BasedOn == nil {
		errs = append(errs, "Task.basedOn is required but not provided")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) createSubTaskEnrollmentCriteria(task *fhir.Task) error {
	questionnaire := s.getHardCodedHomeMonitoringQuestionnaire()

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := s.getEnrollmentCriteriaSubTask(task, questionnaireRef)
	subtaskRef := "urn:uuid:" + subtask["id"].(string)

	tx := coolfhir.Transaction().
		Create(questionnaire, coolfhir.WithFullUrl(questionnaireRef)).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	bundle := tx.Bundle()

	_, err := coolfhir.ExecuteTransaction(s.fhirClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Info().Msg("Successfully created an enrollment subtask")

	return nil
}

func (s *Service) createSubTaskPII(task *fhir.Task) error {
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

	_, err := coolfhir.ExecuteTransaction(s.fhirClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Info().Msg("Successfully created a PII subtask")

	return nil
}

// getEnrollmentCriteriaSubTask creates a new subtask in map[string]interface{} format
// TODO: This doesn't use fhir.Task as the fhir library contains a bug where all possible Task.input[x] are sent to the FHIR client instead of just Task.input.valueReference. This causes either a validation error or not a single Task.input[x] to be set (HAPI)
func (s *Service) getEnrollmentCriteriaSubTask(task *fhir.Task, questionnaireRef string) map[string]interface{} {

	return map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Task",
		"status":       "ready",
		"meta": map[string]interface{}{
			"profile": []string{
				SCP_TASK_PROFILE,
			},
		},
		"basedOn": task.BasedOn,
		"partOf": []map[string]interface{}{
			{
				"reference": "Task/" + *task.Id,
			},
		},
		"focus":     &task.Focus,
		"for":       &task.For,
		"owner":     &task.Requester, // reversed
		"requester": &task.Owner,     // reversed
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
}

// Generates the PII subtask - provide the initial enrollment subtask as argument
// TODO: This doesn't use fhir.Task as the fhir library contains a bug where all possible Task.input[x] are sent to the FHIR client instead of just Task.input.valueReference. This causes either a validation error or not a single Task.input[x] to be set (HAPI)
func (s *Service) getPIISubTask(task *fhir.Task, questionnaireRef string) map[string]interface{} {

	subtask := map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Task",
		"status":       "ready",
		"meta": map[string]interface{}{
			"profile": []string{
				SCP_TASK_PROFILE,
			},
		},
		"basedOn":   &task.BasedOn,
		"partOf":    &task.PartOf,
		"focus":     &task.Focus,
		"for":       &task.For,
		"owner":     &task.Owner,
		"requester": &task.Requester,
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

	log.Info().Msgf("Created a new Enrollment PII subtask for questionnaireRef [%s] - subtask: %v", questionnaireRef, task)

	return subtask
}

func (s *Service) partOf(task *fhir.Task) (*string, error) {
	if len(task.PartOf) == 0 {
		return nil, nil // Optional reference, simply return nil if not set
	}

	partOfRef := task.PartOf[0].Reference
	if partOfRef == nil || !strings.HasPrefix(*partOfRef, "Task/") {
		return nil, errors.New("Task.partOf must contain a relative reference to a Task")
	}

	return partOfRef, nil
}
