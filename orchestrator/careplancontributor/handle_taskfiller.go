package careplancontributor

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleTaskFillerCreate(ctx context.Context, task *fhir.Task) error {
	log.Info().Msgf("Running handleTaskFillerCreate for Task %s", *task.Id)

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
		// Validate that the current CPC is the task Owner to perform task filling
		ids, err := s.profile.Identities(ctx)
		if err != nil {
			return err
		}
		isOwner, _ := coolfhir.ValidateTaskOwnerAndRequester(task, ids)
		if !isOwner {
			log.Info().Msg("Current CPC node is not the task Owner - skipping")
			return nil
		}

		log.Info().Msg("Found a new 'primary' task, checking if more information is needed via a Questionnaire")
		err = s.createSubTaskOrFinishPrimaryTask(task, true)
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
	err = s.carePlanServiceClient.Read(*partOfTaskRef, &partOfTask)
	if err != nil {
		return fmt.Errorf("failed to fetch partOf for %s: %w", *partOfTaskRef, err)
	}

	// Change status to accepted
	partOfTask.Status = fhir.TaskStatusAccepted

	// Update the task in the FHIR server
	err = s.carePlanServiceClient.Update(*partOfTaskRef, &partOfTask, &partOfTask)
	if err != nil {
		return fmt.Errorf("failed to update partOf %s status: %w", *partOfTaskRef, err)
	}

	log.Info().Msgf("Successfully marked %s as completed", *partOfTaskRef)
	return nil
}

func (s *Service) fetchQuestionnaireByID(ref string, questionnaire *fhir.Questionnaire) error {
	log.Debug().Msg("Fetching Questionnaire by ID")

	err := s.carePlanServiceClient.Read(ref, &questionnaire)
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
	subtaskRef := "urn:uuid:" + *subtask.Id

	tx := coolfhir.Transaction().
		Create(questionnaire, coolfhir.WithFullUrl(questionnaireRef)).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	bundle := tx.Bundle()

	_, err = coolfhir.ExecuteTransaction(s.carePlanServiceClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Info().Msg("Successfully created a subtask")

	return nil
}

// getSubTask creates a new subtask providing the questionnaire reference as Task.input.valueReference
func (s *Service) getSubTask(task *fhir.Task, questionnaireRef string, isPrimaryTask bool) fhir.Task {

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

	return fhir.Task{
		Id:     to.Ptr(uuid.NewString()),
		Status: fhir.TaskStatusReady,
		Meta: &fhir.Meta{
			Profile: []string{
				coolfhir.SCPTaskProfile,
			},
		},
		Intent:  "order",
		BasedOn: task.BasedOn,
		PartOf:  partOf,
		Focus:   task.Focus,
		For:     task.For,
		Owner: func() *fhir.Reference {
			if isPrimaryTask {
				return task.Requester // reversed
			}
			return task.Owner
		}(),
		Requester: func() *fhir.Reference {
			if isPrimaryTask {
				return task.Owner // reversed
			}
			return task.Requester
		}(),
		Input: []fhir.TaskInput{
			{
				Type: fhir.CodeableConcept{
					Coding: []fhir.Coding{
						{
							System:  to.Ptr("http://terminology.hl7.org/CodeSystem/task-input-type"),
							Code:    to.Ptr("Reference"),
							Display: to.Ptr("Reference"),
						},
					},
				},
				ValueReference: &fhir.Reference{
					Reference: to.Ptr(questionnaireRef),
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
