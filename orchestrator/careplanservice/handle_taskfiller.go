package careplanservice

import (
	"errors"
	"fmt"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

var primaryTaskFocusToQuestionnaireURL = map[string]string{
	"2.16.528.1.1007.3.3.21514.ehr.orders|99534756439": "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017",
}

var followUpQuestionnaireMap = map[string]string{
	"http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017": "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017",
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

	// If partOfRef is nil, handle the task as a primary task - no need to create follow-up subtasks for newly created Tasks
	//This only happens on Task update where the Task.output is filled with a QuestionnaireResponse
	if partOfRef == nil {
		log.Info().Msg("Found a new 'primary' task, checking if more information is needed via a Questionnaire")
		err := s.createSubTaskOrFinishPrimaryTask(task, true)
		if err != nil {
			return fmt.Errorf("failed to process new primary Task: %w", err)
		}
	}
	return nil
}

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

	return s.createSubTaskOrFinishPrimaryTask(task, false)

}

func (s *Service) markPrimaryTaskAsCompleted(subTask *fhir.Task) error {
	log.Info().Msg("Marking primary Task as completed")

	if subTask.PartOf == nil {
		return errors.New("task.partOf is empty")
	}

	partOfTaskRef := subTask.PartOf[0].Reference
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

func (s *Service) isScpTask(task *fhir.Task) bool {
	if task.Meta == nil {
		return false
	}

	for _, profile := range task.Meta.Profile {
		if profile == coolfhir.SCPTaskProfile {
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

func (s *Service) createSubTaskOrFinishPrimaryTask(task *fhir.Task, isPrimaryTask bool) error {
	if task.Focus == nil || task.Focus.Identifier == nil || task.Focus.Identifier.System == nil || task.Focus.Identifier.Value == nil {
		return errors.New("task.Focus or its Identifier fields are nil")
	}
	taskFocus := fmt.Sprintf("%s|%s", *task.Focus.Identifier.System, *task.Focus.Identifier.Value)
	var questionnaire map[string]interface{}

	if isPrimaryTask {
		questionnaire = s.getQuestionnaireByUrl(primaryTaskFocusToQuestionnaireURL[taskFocus])
	} else {
		//For subtasks, we need to make sure it's completed, and if so, find out if more Questionnaires are needed.
		//We do this by fetching the Questionnaire, and comparing it's url value to the followUpQuestionnaireMap
		if task.Status != fhir.TaskStatusCompleted {
			log.Info().Msg("SubTask is not completed - skipping")
		}

		for _, item := range task.Input {
			if ref := item.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "Questionnaire/") {
				questionnaireURL := *ref.Reference
				var fetchedQuestionnaire fhir.Questionnaire
				err := s.fetchQuestionnaireByID(questionnaireURL, &fetchedQuestionnaire)
				if err != nil {
					return fmt.Errorf("failed to fetch questionnaire: %w", err)
				}
				followUpURL, exists := followUpQuestionnaireMap[*fetchedQuestionnaire.Url]
				if exists {
					questionnaire = s.getQuestionnaireByUrl(followUpURL)
					break
				}
			}
		}
	}

	// No follow-up questionnaire found, check if we have to mark the primary task as completed
	if questionnaire == nil {

		if task.PartOf == nil {
			log.Info().Msg("Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as completed")
			return nil
		}

		for _, output := range task.Output {
			if ref := output.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "QuestionnaireResponse/") {

				var partOfTaskRef *string
				for _, partOf := range task.PartOf {
					if partOf.Reference != nil && strings.HasPrefix(*partOf.Reference, "Task/") {
						partOfTaskRef = partOf.Reference
						break
					}
				}

				if partOfTaskRef == nil {
					return errors.New("no valid Task reference found in task.partOf")
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
				break
			}
		}
		return nil
	}

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := s.getSubTask(task, questionnaireRef, isPrimaryTask)
	subtaskRef := "urn:uuid:" + subtask["id"].(string)

	tx := coolfhir.Transaction().
		Create(questionnaire, coolfhir.WithFullUrl(questionnaireRef)).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	bundle := tx.Bundle()

	_, err := coolfhir.ExecuteTransaction(s.fhirClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Info().Msg("Successfully created a subtask")

	return nil
}

// getSubTask creates a new subtask in map[string]interface{} format
// TODO: This doesn't use fhir.Task as the fhir library contains a bug where all possible Task.input[x] are sent to the FHIR client instead of just Task.input.valueReference. This causes either a validation error or not a single Task.input[x] to be set (HAPI)
func (s *Service) getSubTask(task *fhir.Task, questionnaireRef string, isPrimaryTask bool) map[string]interface{} {

	// By default, point to the Task.partOf, this is used to group the subtasks together under the same primary Task
	partOf := task.PartOf

	// If this is the first subTask for the primary Task, we need to point to the primary Task itself
	if isPrimaryTask {

		partOf = []fhir.Reference{
			{
				Reference: to.Ptr(fmt.Sprintf("Task/%s", *task.Id)),
			},
		}
	}

	return map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Task",
		"status":       "ready",
		"meta": map[string]interface{}{
			"profile": []string{
				coolfhir.SCPTaskProfile,
			},
		},
		"basedOn": task.BasedOn,
		"partOf":  &partOf,
		"focus":   &task.Focus,
		"for":     &task.For,
		"owner": func() *fhir.Reference {
			if isPrimaryTask {
				return task.Requester // reversed
			}
			return task.Owner
		}(),
		"requester": func() *fhir.Reference {
			if isPrimaryTask {
				return task.Owner // reversed
			}
			return task.Requester
		}(),
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
				coolfhir.SCPTaskProfile,
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
