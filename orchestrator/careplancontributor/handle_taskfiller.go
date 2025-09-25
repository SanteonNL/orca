package careplancontributor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/slices"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return err
	}
	if !tenant.TaskEngine.Enabled {
		slog.DebugContext(
			ctx,
			"TaskEngine is disabled for this tenant - skipping Task notification handling",
			slog.String(logging.FieldResourceID, *task.Id),
			slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
		)
		return nil
	}

	slog.InfoContext(ctx, "Running TaskEngine for notification", slog.String(logging.FieldResourceID, *task.Id))
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRTaskID, to.Value(task.Id)),
			attribute.String(otel.FHIRTaskStatus, task.Status.Code()),
		),
	)
	defer span.End()

	slog.InfoContext(
		ctx,
		"Handling Task Notification",
		slog.String(logging.FieldResourceID, *task.Id),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
	)

	if !coolfhir.IsScpTask(task) {
		slog.InfoContext(ctx, "Task is not an SCP Task - skipping")
		span.SetAttributes(attribute.Bool("task.is_scp_task", false))
		span.SetStatus(codes.Ok, "Task is not SCP task, skipping")
		return nil
	}
	span.SetAttributes(attribute.Bool("task.is_scp_task", true))

	if err := s.isValidTask(task); err != nil {
		rejection := TaskRejection{
			Reason:       "Task is not valid",
			ReasonDetail: err,
		}
		return otel.Error(span, rejection, rejection.FormatReason())
	}

	partOfRef, err := s.partOf(task, false)
	if err != nil {
		rejection := TaskRejection{
			Reason:       " Task.partOf is invalid",
			ReasonDetail: err,
		}
		return otel.Error(span, rejection, rejection.FormatReason())
	}

	identities, err := s.profile.Identities(ctx)
	if err != nil {
		return otel.Error(span, err, err.Error())
	}
	localIdentifiers := coolfhir.OrganizationIdentifiers(identities)

	// If partOfRef is nil, handle the task as a primary task - no need to create follow-up subtasks for newly created Tasks
	//This only happens on Task update where the Task.output is filled with a QuestionnaireResponse
	if partOfRef == nil {
		span.SetAttributes(attribute.Bool("task.is_primary_task", true))
		slog.InfoContext(
			ctx,
			"Notified Task is a primary Task",
			slog.String(logging.FieldResourceID, *task.Id),
			slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
		)
		// Check if the primary task is "created", its status will be updated by subtasks that are completed - not directly here
		if task.Status != fhir.TaskStatusRequested {
			slog.DebugContext(ctx, "primary Task.status != requested (workflow already started) - not processing in handleTaskNotification")
			span.SetStatus(codes.Ok, "Primary task status not requested, skipping")
			return nil
		}

		// Validate that the current CPC is the task Owner to perform task filling
		isOwner, _ := coolfhir.IsIdentifierTaskOwnerAndRequester(task, localIdentifiers)
		span.SetAttributes(attribute.Bool("task.is_owner", isOwner))
		if !isOwner {
			slog.InfoContext(ctx, "Current CPC node is not the task Owner - skipping")
			span.SetStatus(codes.Ok, "Current CPC node is not task owner, skipping")
			return nil
		}

		slog.InfoContext(ctx, "Task is a 'primary' task, checking if more information is needed via a Questionnaire, or if we can accept it.")
		err = s.createSubTaskOrAcceptPrimaryTask(ctx, cpsClient, task, task, localIdentifiers)
		if err != nil {
			return otel.Error(span, fmt.Errorf("failed to process new primary Task: %w", err), err.Error())
		}
	} else {
		span.SetAttributes(
			attribute.Bool("task.is_primary_task", false),
			attribute.String("task.primary_task_ref", *partOfRef),
		)
		slog.InfoContext(
			ctx,
			"Notified Task is a sub-task",
			slog.String(logging.FieldResourceID, *task.Id),
			slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
			slog.String("primary_task_reference", *partOfRef),
		)
		err = s.handleSubtaskNotification(ctx, cpsClient, task, *partOfRef, localIdentifiers)
		if err != nil {
			return otel.Error(span, fmt.Errorf("failed to update sub Task: %w", err), err.Error())
		}
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// TODO: This function now always expects a subtask, but it should also be able to handle primary tasks
func (s *Service) handleSubtaskNotification(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task, primaryTaskRef string, identities []fhir.Identifier) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRTaskID, to.Value(task.Id)),
			attribute.String(otel.FHIRTaskStatus, task.Status.Code()),
			attribute.String("fhir.primary_task_ref", primaryTaskRef),
		),
	)
	defer span.End()

	if task.Status != fhir.TaskStatusCompleted {
		slog.DebugContext(ctx, "Task.status is not completed - skipping")
		span.SetStatus(codes.Ok, "Task status not completed, skipping")
		return nil
	}
	slog.InfoContext(ctx, "SubTask.status is completed - processing")

	primaryTask := new(fhir.Task)
	err := cpsClient.Read(primaryTaskRef, primaryTask)
	if err != nil {
		rejection := &TaskRejection{
			Reason:       "Processing failed",
			ReasonDetail: fmt.Errorf("failed to fetch primary Task of subtask (subtask.id=%s, primarytask.ref=%s): %w", *task.Id, primaryTaskRef, err),
		}
		return otel.Error(span, rejection, rejection.FormatReason())
	}
	span.SetAttributes(attribute.String("fhir.primary_task_id", to.Value(primaryTask.Id)))

	if coolfhir.IsScpSubTask(primaryTask) {
		rejection := &TaskRejection{
			Reason:       "Invalid Task",
			ReasonDetail: errors.New("sub-task references another sub-task. Nested subtasks are not supported"),
		}
		return otel.Error(span, rejection, rejection.FormatReason())
	}

	err = s.createSubTaskOrAcceptPrimaryTask(ctx, cpsClient, task, primaryTask, identities)
	if err != nil {
		return otel.Error(span, err, err.Error())
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (s *Service) acceptPrimaryTask(ctx context.Context, cpsClient fhirclient.Client, primaryTask *fhir.Task) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRTaskID, to.Value(primaryTask.Id)),
			attribute.String(otel.FHIRTaskStatus, primaryTask.Status.Code()),
		),
	)
	defer span.End()

	slog.DebugContext(
		ctx,
		"Started function acceptPrimaryTask() for Task",
		slog.String(logging.FieldResourceID, *primaryTask.Id),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
	)
	if primaryTask.Status != fhir.TaskStatusRequested && primaryTask.Status != fhir.TaskStatusReceived {
		slog.DebugContext(ctx, "primary Task.status != requested||received (workflow already started) - not processing in handleTaskNotification")
		span.SetStatus(codes.Ok, "Task status not requested or received, skipping")
		return nil
	}
	ref := "Task/" + *primaryTask.Id
	span.SetAttributes(attribute.String("fhir.task_ref", ref))

	slog.InfoContext(
		ctx,
		"TaskEngine: Accepting primary Task",
		slog.String(logging.FieldResourceReference, ref),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
		slog.String(logging.FieldResourceID, *primaryTask.Id),
	)
	primaryTask.Status = fhir.TaskStatusAccepted
	if note := s.getTaskStatusNote(primaryTask.Status); note != nil {
		primaryTask.Note = append(primaryTask.Note, fhir.Annotation{
			Text: *note,
		})
	}
	// Update the task in the FHIR server
	err := cpsClient.Update(ref, primaryTask, primaryTask)
	if err != nil {
		return otel.Error(span, fmt.Errorf("failed to update primary Task status (id=%s): %w", ref, err), err.Error())
	}
	slog.DebugContext(
		ctx,
		"Successfully accepted task",
		slog.String(logging.FieldResourceReference, ref),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
	)

	notificationSent := false
	if s.notifier != nil {
		slog.InfoContext(
			ctx,
			"TaskEngine: EHR will be notified of accepted Task with bundle of relevant FHIR resources",
			slog.String(logging.FieldResourceReference, ref),
			slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
		)
		err = s.notifier.NotifyTaskAccepted(ctx, cpsClient.Path().String(), primaryTask)
		if err != nil {
			slog.WarnContext(
				ctx,
				"Accepted Task with an error in the notification",
				slog.String(logging.FieldResourceReference, ref),
				slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
				slog.String(logging.FieldError, otel.Error(span, err, err.Error()).Error()),
			)
			return nil
		} else {
			notificationSent = true
		}
	}
	span.SetAttributes(attribute.Bool("notification.sent", notificationSent))

	slog.DebugContext(
		ctx,
		"Successfully accepted Task",
		slog.String(logging.FieldResourceReference, ref),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
	)

	span.SetStatus(codes.Ok, "")
	return nil
}

func (s *Service) fetchQuestionnaireByID(ctx context.Context, cpsClient fhirclient.Client, ref string, questionnaire *fhir.Questionnaire) error {
	slog.DebugContext(
		ctx,
		"Fetching Questionnaire by ID",
		slog.String(logging.FieldResourceReference, ref),
		slog.String(logging.FieldResourceType, fhir.ResourceTypeQuestionnaire.String()),
	)
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
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRTaskID, to.Value(task.Id)),
			attribute.String("fhir.primary_task_id", to.Value(primaryTask.Id)),
		),
	)
	defer span.End()

	// Look up primary Task: workflow selection works on primary Task.reasonCode/reasonReference
	isPrimaryTask := *task.Id == *primaryTask.Id
	span.SetAttributes(attribute.Bool("task.is_primary_task", isPrimaryTask))

	var questionnaire *fhir.Questionnaire
	workflow, err := s.selectWorkflow(ctx, cpsClient, primaryTask)
	if err != nil {
		rejection := &TaskRejection{
			Reason: err.Error(),
		}
		return otel.Error(span, rejection, rejection.FormatReason())
	}

	workflowFound := workflow != nil
	span.SetAttributes(attribute.Bool("workflow.found", workflowFound))

	if workflow != nil {
		var nextStep *taskengine.WorkflowStep
		if isPrimaryTask {
			nextStep = new(taskengine.WorkflowStep)
			*nextStep = workflow.Start()
			span.SetAttributes(attribute.String("workflow.step_type", "start"))
		} else {
			// For subtasks, we need to make sure it's completed, and if so, find out if more Questionnaires are needed.
			// We do this by fetching the Questionnaire, and comparing it's url value to the followUpQuestionnaireMap
			if task.Status != fhir.TaskStatusCompleted {
				slog.InfoContext(ctx, "SubTask is not completed - skipping")
			}
			// TODO: Should we check if there's actually a QuestionnaireResponse?
			// TODO: What if multiple Tasks match the conditions?
			for _, item := range task.Input {
				if ref := item.ValueReference; ref.Reference != nil && strings.HasPrefix(*ref.Reference, "Questionnaire/") {
					questionnaireURL := *ref.Reference
					span.SetAttributes(attribute.String("questionnaire.previous_url", questionnaireURL))
					var fetchedQuestionnaire fhir.Questionnaire
					if err := s.fetchQuestionnaireByID(ctx, cpsClient, questionnaireURL, &fetchedQuestionnaire); err != nil {
						// TODO: why return an error here, and not for the rest?
						rejection := &TaskRejection{
							Reason:       "Failed to fetch questionnaire",
							ReasonDetail: err,
						}
						return otel.Error(span, rejection, rejection.FormatReason())
					}
					nextStep, err = workflow.Proceed(*item.ValueReference.Reference)
					if err != nil {
						slog.ErrorContext(
							ctx,
							"Unable to determine next questionnaire",
							slog.String("previous_url", *fetchedQuestionnaire.Url),
							slog.String(logging.FieldError, err.Error()),
						)
					} else {
						span.SetAttributes(attribute.String("workflow.step_type", "proceed"))
						break
					}
				}
			}
		}

		// TODO: If we can't perform the next step, we should mark the primary task as failed?
		if nextStep != nil {
			span.SetAttributes(attribute.String("questionnaire.next_url", nextStep.QuestionnaireUrl))
			slog.DebugContext(ctx, "Found next step in workflow, loading questionnaire", slog.String("next_url", nextStep.QuestionnaireUrl))
			questionnaire, err = s.workflows.QuestionnaireLoader().Load(ctx, nextStep.QuestionnaireUrl)
			if err != nil {
				rejection := &TaskRejection{
					Reason:       "Failed to load questionnaire: " + nextStep.QuestionnaireUrl,
					ReasonDetail: err,
				}
				return otel.Error(span, rejection, rejection.FormatReason())
			}
		}
	}

	// No follow-up questionnaire found, check if we can accept the primary Task
	if questionnaire == nil {
		span.SetAttributes(attribute.String("task.outcome", "accept_or_skip"))

		if task.PartOf == nil {
			// TODO: reject here: nothing more to do
			slog.InfoContext(ctx, "Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as accepted")
			rejection := &TaskRejection{
				Reason: "Did not find a follow-up questionnaire, and the task has no partOf set - cannot mark primary task as accepted",
			}
			return otel.Error(span, rejection, rejection.FormatReason())
		}

		// TODO: Only accept main task is in status 'requested'
		isPrimaryTaskOwner, _ := coolfhir.IsIdentifierTaskOwnerAndRequester(primaryTask, localOrgIdentifiers)
		span.SetAttributes(attribute.Bool("task.is_primary_task_owner", isPrimaryTaskOwner))
		if isPrimaryTaskOwner {
			err := s.acceptPrimaryTask(ctx, cpsClient, primaryTask)
			if err != nil {
				return otel.Error(span, err, err.Error())
			} else {
				span.SetStatus(codes.Ok, "Primary task accepted")
			}
			return err
		} else {
			span.SetStatus(codes.Ok, "Not primary task owner, skipping")
			return nil
		}
	}

	// Create a new SubTask based on the Questionnaire reference
	span.SetAttributes(
		attribute.String("task.outcome", "create_subtask"),
		attribute.String("questionnaire.id", to.Value(questionnaire.Id)),
	)
	questionnaireRef := "Questionnaire/" + *questionnaire.Id
	subtask := s.getSubTask(primaryTask, questionnaireRef)
	subtaskRef := "urn:uuid:" + *subtask.Id
	span.SetAttributes(attribute.String("fhir.subtask_id", to.Value(subtask.Id)))

	tx := coolfhir.Transaction().
		Update(questionnaire, questionnaireRef).
		Create(subtask, coolfhir.WithFullUrl(subtaskRef))

	if isPrimaryTask && primaryTask.Status == fhir.TaskStatusRequested {
		// Mark the task as "received" to indicate that the task is being processed
		slog.InfoContext(
			ctx,
			"Marking task as received",
			slog.String(logging.FieldResourceID, *task.Id),
			slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
		)
		primaryTask.Status = fhir.TaskStatusReceived
		tx.Update(primaryTask, "Task/"+*primaryTask.Id)
		span.SetAttributes(attribute.Bool("task.primary_task_updated", true))
	}

	bundle := tx.Bundle()

	_, err = coolfhir.ExecuteTransaction(cpsClient, bundle)
	if err != nil {
		return otel.Error(span, fmt.Errorf("failed to execute transaction: %w", err), err.Error())
	}

	slog.InfoContext(ctx, "Successfully created a subtask")

	span.SetStatus(codes.Ok, "")
	return nil
}

// selectWorkflow determines the workflow to use based on the Task's focus, and reasonCode or reasonReference.
// It first selects the type of service, from the Task.focus (ServiceRequest), and then selects the workflow based on the Task.reasonCode or Task.reasonReference.
// If it finds no, or multiple, matching workflows, it returns an error.
func (s *Service) selectWorkflow(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) (*taskengine.Workflow, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRTaskID, to.Value(task.Id)),
			attribute.String("fhir.task_focus_reference", to.Value(task.Focus.Reference)),
		),
	)
	defer span.End()

	// Determine service code from Task.focus
	var serviceRequest fhir.ServiceRequest
	if err := cpsClient.Read(*task.Focus.Reference, &serviceRequest); err != nil {
		return nil, otel.Error(span, fmt.Errorf("failed to fetch ServiceRequest (path=%s, task=%s): %w", *task.Focus.Reference, *task.Id, err), err.Error())
	}
	span.SetAttributes(attribute.String("fhir.service_request_id", to.Value(serviceRequest.Id)))

	// Determine reason codes from Task.reasonCode and Task.reasonReference
	var taskReasonCodes []fhir.Coding
	if task.ReasonCode != nil {
		taskReasonCodes = task.ReasonCode.Coding
		span.SetAttributes(attribute.Int("task.reason_codes_from_direct", len(taskReasonCodes)))
	}
	if task.ReasonReference != nil && task.ReasonReference.Reference != nil {
		span.SetAttributes(attribute.String("fhir.reason_reference", to.Value(task.ReasonReference.Reference)))
		var condition fhir.Condition
		if err := cpsClient.Read(*task.ReasonReference.Reference, &condition); err != nil {
			return nil, otel.Error(span, fmt.Errorf("failed to fetch Condition of Task.reasonReference.reference (path=%s, task=%s): %w", *task.ReasonReference.Reference, *task.Id, err), err.Error())
		}
		span.SetAttributes(attribute.String("fhir.condition_id", to.Value(condition.Id)))
		reasonCodesFromCondition := 0
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
				reasonCodesFromCondition++
			}
		}
		span.SetAttributes(attribute.Int("task.reason_codes_from_condition", reasonCodesFromCondition))
	}
	taskReasonCodes = slices.Deduplicate(taskReasonCodes, func(a, b fhir.Coding) bool {
		return *a.System == *b.System && *a.Code == *b.Code
	})
	span.SetAttributes(attribute.Int("task.total_reason_codes", len(taskReasonCodes)))

	var matchedWorkflows []*taskengine.Workflow
	workflowLookups := 0
	for _, serviceCoding := range serviceRequest.Code.Coding {
		if serviceCoding.System == nil || serviceCoding.Code == nil {
			continue
		}
		for _, reasonCoding := range taskReasonCodes {
			workflowLookups++
			workflow, err := s.workflows.Provide(ctx, serviceCoding, reasonCoding)
			if errors.Is(err, taskengine.ErrWorkflowNotFound) {
				slog.DebugContext(ctx, "No workflow found",
					slog.String(logging.FieldError, err.Error()),
					slog.String("service", fmt.Sprintf("%s|%s", *serviceCoding.System, *serviceCoding.Code)),
					slog.String("condition", fmt.Sprintf("%s|%s", *reasonCoding.System, *reasonCoding.Code)),
					slog.String(logging.FieldResourceID, *task.Id),
					slog.String(logging.FieldResourceType, fhir.ResourceTypeTask.String()),
				)
				continue
			} else if err != nil {
				// Other error occurred
				return nil, otel.Error(span, fmt.Errorf("workflow lookup (service=%s|%s, condition=%s|%s, task=%s): %w", *serviceCoding.System, *serviceCoding.Code, *reasonCoding.System, *reasonCoding.Code, *task.Id, err), err.Error())
			}
			matchedWorkflows = append(matchedWorkflows, workflow)
		}
	}

	span.SetAttributes(
		attribute.Int("workflow.lookups_performed", workflowLookups),
		attribute.Int("workflow.matches_found", len(matchedWorkflows)),
	)

	if len(matchedWorkflows) == 0 {
		return nil, otel.Error(span, fmt.Errorf("ServiceRequest.code and Task.reason.code does not match any workflows (task=%s)", *task.Id), "no matching workflows found")
	} else if len(matchedWorkflows) > 1 {
		return nil, otel.Error(span, fmt.Errorf("ServiceRequest.code and Task.reason.code matches multiple workflows, need to choose one (task=%s)", *task.Id), "multiple workflows matched")
	}

	span.SetStatus(codes.Ok, "")
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
