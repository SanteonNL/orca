package careplanservice

import (
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// TODO: Move to CarePlanContributor as TaskEngine, invoked by the CPS notification
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

	partOfRef, err := s.partOf(task, false)
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

// TODO: This function now always expects a subtask, but it should also be able to handle primary tasks
func (s *Service) handleTaskFillerUpdate(task *fhir.Task) error {

	log.Info().Msg("Running handleTaskFillerUpdate")

	if !s.isScpTask(task) {
		log.Debug().Msg("Task is not an SCP Task - skipping")
		return nil
	}

	if task.Status != fhir.TaskStatusCompleted {
		log.Debug().Msg("Task.status is not completed - skipping")
		return nil
	}

	if err := s.isValidTask(task); err != nil {
		log.Warn().Err(err).Msg("Task invalid - skipping")
		return fmt.Errorf("task is not valid - skipping: %w", err)
	}

	if _, err := s.partOf(task, true); err != nil {
		return fmt.Errorf("expected a subTask - failed to extract Task.partOf for Task/%s: %w", *task.Id, err)
	}

	log.Info().Msg("SubTask.status is completed - processing")

	return s.createSubTaskOrFinishPrimaryTask(task, false)

}

func (s *Service) markPrimaryTaskAsCompleted(subTask *fhir.Task) error {
	log.Debug().Msg("Marking primary Task as completed")

	partOfTaskRef, err := s.partOf(subTask, true)
	if err != nil {
		return err
	}

	var partOfTask fhir.Task
	err = s.fhirClient.Read(*partOfTaskRef, &partOfTask)
	if err != nil {
		return fmt.Errorf("failed to fetch partOf Task/%s: %w", *partOfTaskRef, err)
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
	log.Debug().Msg("Fetching Questionnaire by ID")

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

	workflow, workflowExists := s.workflows[taskFocus]
	var err error
	if workflowExists {
		var nextStep *taskengine.WorkflowStep
		if isPrimaryTask {
			nextStep = new(taskengine.WorkflowStep)
			*nextStep = workflow.Start()
		} else {
			//For subtasks, we need to make sure it's completed, and if so, find out if more Questionnaires are needed.
			//We do this by fetching the Questionnaire, and comparing it's url value to the followUpQuestionnaireMap
			if task.Status != fhir.TaskStatusCompleted {
				log.Info().Msg("SubTask is not completed - skipping")
			}
			// TODO: What if multiple Tasks match the conditions?
			for _, item := range task.Input {
				if ref := item.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "Questionnaire/") {
					questionnaireURL := *ref.Reference
					var fetchedQuestionnaire fhir.Questionnaire
					if err := s.fetchQuestionnaireByID(questionnaireURL, &fetchedQuestionnaire); err != nil {
						// TODO: why return an error here, and not for the rest?
						return fmt.Errorf("failed to fetch questionnaire: %w", err)
					}
					nextStep, err = workflow.Proceed(*fetchedQuestionnaire.Url)
					if err != nil {
						log.Error().Err(err).Msgf("Unable to determine next questionnaire (previous URL=%s)", *fetchedQuestionnaire.Url)
					} else {
						break
					}
				}
			}
		}

		// TODO: If we can't perform the next step, we should mark the primary task as failed?
		if nextStep != nil {
			questionnaire, err = s.questionnaireLoader.Load(nextStep.QuestionnaireUrl)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to load questionnaire: %s", workflow.Steps[0].QuestionnaireUrl)
			}
		}

	}

	// No follow-up questionnaire found, check if we have to mark the primary task as completed
	if questionnaire == nil {

		if task.PartOf == nil {
			log.Info().Msg("Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as completed")
			return nil
		}

		return s.markPrimaryTaskAsCompleted(task)
	}

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := s.getSubTask(task, questionnaireRef, isPrimaryTask)
	subtaskRef := "urn:uuid:" + subtask["id"].(string)

	tx := coolfhir.Transaction().
		Create(questionnaire, coolfhir.WithFullUrl(questionnaireRef)).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	bundle := tx.Bundle()

	_, err = coolfhir.ExecuteTransaction(s.fhirClient, bundle)
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

func (s *Service) partOf(task *fhir.Task, partOfRequired bool) (*string, error) {
	if len(task.PartOf) == 0 {
		if !partOfRequired {
			return nil, nil // Optional reference, not required in "primary" Tasks. simply return nil if not set
		}

		return nil, errors.New("Task.partOf is required but not provided")
	}

	partOfRef := task.PartOf[0].Reference
	if partOfRef == nil || !strings.HasPrefix(*partOfRef, "Task/") {
		return nil, errors.New("Task.partOf must contain a relative reference to a Task")
	}

	return partOfRef, nil
}
