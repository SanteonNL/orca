package careplancontributor

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/slices"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var _ error = TaskRejection{}

// TaskRejection is an error type that is used when a Task can't be processed by the Task Filler, and isn't retryable.
// Reasons are: invalid Task, missing Task.partOf, unsupported service or condition code, etc.
// It should NOT be used for transient errors, like network issues: in that case, the Task should be retried.
type TaskRejection struct {
	Reason       string
	ReasonDetail error
}

func (t TaskRejection) FormatReason() string {
	if t.ReasonDetail != nil {
		return fmt.Sprintf("%s: %s", t.Reason, t.ReasonDetail.Error())
	}
	return t.Reason
}

func (t TaskRejection) Error() string {
	return "task rejected by filler: " + t.FormatReason()
}

func (s *Service) handleTaskNotification(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error {
	log.Ctx(ctx).Info().Msgf("Running handleTaskNotification for Task %s", *task.Id)

	if !coolfhir.IsScpTask(task) {
		log.Ctx(ctx).Info().Msg("Task is not an SCP Task - skipping")
		return nil
	}

	if err := s.isValidTask(task); err != nil {
		return TaskRejection{
			Reason:       "Task is not valid",
			ReasonDetail: err,
		}
	}

	partOfRef, err := s.partOf(task, false)
	if err != nil {
		return TaskRejection{
			Reason:       " Task.partOf is invalid",
			ReasonDetail: err,
		}
	}

	identities, err := s.profile.Identities(ctx)
	if err != nil {
		return err
	}
	localIdentifiers := coolfhir.OrganizationIdentifiers(identities)

	// If partOfRef is nil, handle the task as a primary task - no need to create follow-up subtasks for newly created Tasks
	//This only happens on Task update where the Task.output is filled with a QuestionnaireResponse
	if partOfRef == nil {
		log.Ctx(ctx).Info().Msgf("Notified Task is a primary Task (id=%s)", *task.Id)
		// Check if the primary task is "created", its status will be updated by subtasks that are completed - not directly here
		if task.Status != fhir.TaskStatusRequested {
			log.Ctx(ctx).Debug().Msg("primary Task.status != requested (workflow already started) - not processing in handleTaskNotification")
			return nil
		}

		// Validate that the current CPC is the task Owner to perform task filling
		isOwner, _ := coolfhir.IsIdentifierTaskOwnerAndRequester(task, localIdentifiers)
		if !isOwner {
			log.Ctx(ctx).Info().Msg("Current CPC node is not the task Owner - skipping")
			return nil
		}

		log.Ctx(ctx).Info().Msg("Task is a 'primary' task, checking if more information is needed via a Questionnaire, or if we can accept it.")
		err = s.createSubTaskOrAcceptPrimaryTask(ctx, cpsClient, task, task, localIdentifiers)
		if err != nil {
			return fmt.Errorf("failed to process new primary Task: %w", err)
		}
	} else {
		log.Ctx(ctx).Info().Msgf("Notified Task is a sub-task (id=%s, primary task=%s)", *task.Id, *partOfRef)
		err = s.handleSubtaskNotification(ctx, cpsClient, task, *partOfRef, localIdentifiers)
		if err != nil {
			return fmt.Errorf("failed to update sub Task: %w", err)
		}
	}
	return nil
}

// TODO: This function now always expects a subtask, but it should also be able to handle primary tasks
func (s *Service) handleSubtaskNotification(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task, primaryTaskRef string, identities []fhir.Identifier) error {
	if task.Status != fhir.TaskStatusCompleted {
		log.Ctx(ctx).Debug().Msg("Task.status is not completed - skipping")
		return nil
	}
	log.Ctx(ctx).Info().Msg("SubTask.status is completed - processing")

	primaryTask := new(fhir.Task)
	err := cpsClient.Read(primaryTaskRef, primaryTask)
	if err != nil {
		return &TaskRejection{
			Reason:       "Processing failed",
			ReasonDetail: fmt.Errorf("failed to fetch primary Task of subtask (subtask.id=%s, primarytask.ref=%s): %w", *task.Id, primaryTaskRef, err),
		}
	}

	if coolfhir.IsScpSubTask(primaryTask) {
		return &TaskRejection{
			Reason:       "Invalid Task",
			ReasonDetail: errors.New("sub-task references another sub-task. Nested subtasks are not supported"),
		}
	}

	return s.createSubTaskOrAcceptPrimaryTask(ctx, cpsClient, task, primaryTask, identities)

}

func (s *Service) acceptPrimaryTask(ctx context.Context, cpsClient fhirclient.Client, primaryTask *fhir.Task) error {
	log.Ctx(ctx).Debug().Msgf("Started function acceptPrimaryTask() for Task (task=%s)", *primaryTask.Id)
	if primaryTask.Status != fhir.TaskStatusRequested && primaryTask.Status != fhir.TaskStatusReceived {
		log.Ctx(ctx).Debug().Msg("primary Task.status != requested||received (workflow already started) - not processing in handleTaskNotification")
		return nil
	}
	ref := "Task/" + *primaryTask.Id
	log.Ctx(ctx).Info().Msgf("TaskEngine: Accepting primary Task (task=%s)", ref)
	primaryTask.Status = fhir.TaskStatusAccepted
	if note := s.getTaskStatusNote(primaryTask.Status); note != nil {
		primaryTask.Note = append(primaryTask.Note, fhir.Annotation{
			Text: *note,
		})
	}
	// Update the task in the FHIR server
	err := cpsClient.Update(ref, primaryTask, primaryTask)
	if err != nil {
		return fmt.Errorf("failed to update primary Task status (id=%s): %w", ref, err)
	}
	log.Ctx(ctx).Debug().Msgf("Successfully accepted task (ref=%s)", ref)
	if s.notifier != nil {
		log.Ctx(ctx).Info().Msgf("TaskEngine: EHR will be notified of accepted Task with bundle of relevant FHIR resources (task=%s)", ref)
		err = s.notifier.NotifyTaskAccepted(ctx, cpsClient.Path().String(), primaryTask)
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("Accepted Task with an error in the notification (task=%s): %s", ref, err.Error())
			return nil
		}
	}
	log.Ctx(ctx).Debug().Msgf("Successfully accepted Task (ref=%s)", ref)
	return nil
}

func (s *Service) fetchQuestionnaireByID(ctx context.Context, cpsClient fhirclient.Client, ref string, questionnaire *fhir.Questionnaire) error {
	log.Ctx(ctx).Debug().Msg("Fetching Questionnaire by ID")
	err := cpsClient.Read(ref, &questionnaire)
	if err != nil {
		return fmt.Errorf("failed to fetch Questionnaire: %s", err.Error())
	}
	return nil
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
	// TODO: We only support task.Focus with a literal reference for now, so no logical identifiers
	if task.Focus == nil {
		errs = append(errs, "Task.Focus is required but not provided")
	} else if task.Focus.Reference == nil {
		errs = append(errs, "Task.Focus.reference is required but not provided")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) createSubTaskOrAcceptPrimaryTask(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task, primaryTask *fhir.Task, localOrgIdentifiers []fhir.Identifier) error {
	// Look up primary Task: workflow selection works on primary Task.reasonCode/reasonReference
	isPrimaryTask := *task.Id == *primaryTask.Id

	var questionnaire *fhir.Questionnaire
	workflow, err := s.selectWorkflow(ctx, cpsClient, primaryTask)
	if err != nil {
		return &TaskRejection{
			Reason: err.Error(),
		}
	}
	if workflow != nil {
		var nextStep *taskengine.WorkflowStep
		if isPrimaryTask {
			nextStep = new(taskengine.WorkflowStep)
			*nextStep = workflow.Start()
		} else {
			// For subtasks, we need to make sure it's completed, and if so, find out if more Questionnaires are needed.
			// We do this by fetching the Questionnaire, and comparing it's url value to the followUpQuestionnaireMap
			if task.Status != fhir.TaskStatusCompleted {
				log.Ctx(ctx).Info().Msg("SubTask is not completed - skipping")
			}
			// TODO: Should we check if there's actually a QuestionnaireResponse?
			// TODO: What if multiple Tasks match the conditions?
			for _, item := range task.Input {
				if ref := item.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "Questionnaire/") {
					questionnaireURL := *ref.Reference
					var fetchedQuestionnaire fhir.Questionnaire
					if err := s.fetchQuestionnaireByID(ctx, cpsClient, questionnaireURL, &fetchedQuestionnaire); err != nil {
						// TODO: why return an error here, and not for the rest?
						return &TaskRejection{
							Reason:       "Failed to fetch questionnaire",
							ReasonDetail: err,
						}
					}
					nextStep, err = workflow.Proceed(*item.ValueReference.Reference)
					if err != nil {
						log.Ctx(ctx).Error().Err(err).Msgf("Unable to determine next questionnaire (previous URL=%s)", *fetchedQuestionnaire.Url)
					} else {
						break
					}
				}
			}
		}

		// TODO: If we can't perform the next step, we should mark the primary task as failed?
		if nextStep != nil {
			log.Ctx(ctx).Debug().Msgf("Found next step in workflow, loading questionnaire (url=%s)", nextStep.QuestionnaireUrl)
			questionnaire, err = s.workflows.QuestionnaireLoader().Load(ctx, nextStep.QuestionnaireUrl)
			if err != nil {
				return &TaskRejection{
					Reason:       "Failed to load questionnaire: " + nextStep.QuestionnaireUrl,
					ReasonDetail: err,
				}
			}
		}

	}

	// No follow-up questionnaire found, check if we can accept the primary Task
	if questionnaire == nil {

		if task.PartOf == nil {
			// TODO: reject here: nothing more to do
			log.Ctx(ctx).Info().Msg("Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as accepted")
			return &TaskRejection{
				Reason: "Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as accepted",
			}
		}

		// TODO: Only accept main task is in status 'requested'
		isPrimaryTaskOwner, _ := coolfhir.IsIdentifierTaskOwnerAndRequester(primaryTask, localOrgIdentifiers)
		if isPrimaryTaskOwner {
			return s.acceptPrimaryTask(ctx, cpsClient, primaryTask)
		} else {
			return nil
		}
	}

	// Create a new SubTask based on the Questionnaire reference
	questionnaireRef := "Questionnaire/" + *questionnaire.Id
	subtask := s.getSubTask(primaryTask, questionnaireRef)
	subtaskRef := "urn:uuid:" + *subtask.Id

	tx := coolfhir.Transaction().
		Update(questionnaire, questionnaireRef).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	if isPrimaryTask && primaryTask.Status == fhir.TaskStatusRequested {
		// Mark the task as "received" to indicate that the task is being processed
		log.Ctx(ctx).Info().Msgf("Marking task as received (id=%s)", *task.Id)
		primaryTask.Status = fhir.TaskStatusReceived
		tx.Update(primaryTask, "Task/"+*primaryTask.Id)
	}

	bundle := tx.Bundle()

	_, err = coolfhir.ExecuteTransaction(cpsClient, bundle)
	if err != nil {
		return fmt.Errorf("failed to execute transaction: %w", err)
	}

	log.Ctx(ctx).Info().Msg("Successfully created a subtask")

	return nil
}

// selectWorkflow determines the workflow to use based on the Task's focus, and reasonCode or reasonReference.
// It first selects the type of service, from the Task.focus (ServiceRequest), and then selects the workflow based on the Task.reasonCode or Task.reasonReference.
// If it finds no, or multiple, matching workflows, it returns an error.
func (s *Service) selectWorkflow(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) (*taskengine.Workflow, error) {
	// Determine service code from Task.focus
	var serviceRequest fhir.ServiceRequest
	if err := cpsClient.Read(*task.Focus.Reference, &serviceRequest); err != nil {
		return nil, fmt.Errorf("failed to fetch ServiceRequest (path=%s, task=%s): %w", *task.Focus.Reference, *task.Id, err)
	}

	// Determine reason codes from Task.reasonCode and Task.reasonReference
	var taskReasonCodes []fhir.Coding
	if task.ReasonCode != nil {
		taskReasonCodes = task.ReasonCode.Coding
	}
	if task.ReasonReference != nil && task.ReasonReference.Reference != nil {
		var condition fhir.Condition
		if err := cpsClient.Read(*task.ReasonReference.Reference, &condition); err != nil {
			return nil, fmt.Errorf("failed to fetch Condition of Task.reasonReference.reference (path=%s, task=%s): %w", *task.ReasonReference.Reference, *task.Id, err)
		}
		for _, coding := range condition.Code.Coding {
			if coding.System == nil || coding.Code == nil {
				continue
			}
			var present bool
			for _, taskReasonCode := range taskReasonCodes {
				if taskReasonCode.System == coding.System && taskReasonCode.Code == coding.Code {
					present = true
					break
				}
			}
			if !present {
				taskReasonCodes = append(taskReasonCodes, coding)
			}
		}
	}
	taskReasonCodes = slices.Deduplicate(taskReasonCodes, func(a, b fhir.Coding) bool {
		return *a.System == *b.System && *a.Code == *b.Code
	})

	var matchedWorkflows []*taskengine.Workflow
	for _, serviceCoding := range serviceRequest.Code.Coding {
		if serviceCoding.System == nil || serviceCoding.Code == nil {
			continue
		}
		for _, reasonCoding := range taskReasonCodes {
			workflow, err := s.workflows.Provide(ctx, serviceCoding, reasonCoding)
			if errors.Is(err, taskengine.ErrWorkflowNotFound) {
				log.Ctx(ctx).Debug().Err(err).Msgf("No workflow found (service=%s|%s, condition=%s|%s, task=%s)",
					*serviceCoding.System, *serviceCoding.Code, *reasonCoding.System, *reasonCoding.Code, *task.Id)
				continue
			} else if err != nil {
				// Other error occurred
				return nil, fmt.Errorf("workflow lookup (service=%s|%s, condition=%s|%s, task=%s): %w", *serviceCoding.System, *serviceCoding.Code, *reasonCoding.System, *reasonCoding.Code, *task.Id, err)
			}
			matchedWorkflows = append(matchedWorkflows, workflow)
		}
	}
	if len(matchedWorkflows) == 0 {
		return nil, fmt.Errorf("ServiceRequest.code and Task.reason.code does not match any workflows (task=%s)", *task.Id)
	} else if len(matchedWorkflows) > 1 {
		return nil, fmt.Errorf("ServiceRequest.code and Task.reason.code matches multiple workflows, need to choose one (task=%s)", *task.Id)
	}
	return matchedWorkflows[0], nil
}

// getSubTask creates a new subtask providing the questionnaire reference as Task.input.valueReference
func (s *Service) getSubTask(parentTask *fhir.Task, questionnaireRef string) fhir.Task {
	return fhir.Task{
		Id:     to.Ptr(uuid.NewString()),
		Status: fhir.TaskStatusReady,
		Meta: &fhir.Meta{
			Profile: []string{
				coolfhir.SCPTaskProfile,
			},
		},
		Intent:  "order",
		BasedOn: parentTask.BasedOn,
		PartOf: []fhir.Reference{
			{
				Reference: to.Ptr(fmt.Sprintf("Task/%s", *parentTask.Id)),
			},
		},
		Focus:     parentTask.Focus,
		For:       parentTask.For,
		Owner:     parentTask.Requester, // reversed
		Requester: parentTask.Owner,     // reversed
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

func (s *Service) getTaskStatusNote(status fhir.TaskStatus) *string {
	// remove all non A-Z characters
	mapKey := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r
		}
		return -1
	}, status.Code())
	if note, ok := s.config.TaskFiller.StatusNote[mapKey]; ok {
		return &note
	}
	return nil
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
