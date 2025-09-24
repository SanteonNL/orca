package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (s *Service) handleUpdateTask(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.FHIRResourceType, "Task"),
		),
	)
	defer span.End()

	slog.InfoContext(ctx, "Updating Task", slog.Any("url", request.RequestUrl))
	var task fhir.Task
	var err error
	if err = json.Unmarshal(request.ResourceData, &task); err != nil {
		return nil, otel.Error(span, fmt.Errorf("invalid %T: %w", task, coolfhir.BadRequestError(err)), "failed to unmarshal task")
	}

	// Task is owned by CPS, don't allow changing or setting the source of the Task
	if task.Meta != nil {
		task.Meta.Source = nil
	}

	// Add task status to span attributes for better observability
	if task.Status.String() != "" {
		span.SetAttributes(attribute.String("fhir.task.status", task.Status.String()))
	}

	// Check we're only allowing secure external literal references
	if err = validateLiteralReferences(ctx, s.profile, &task); err != nil {
		return nil, otel.Error(span, err, "literal reference validation failed")
	}

	// Validate fields on updated Task
	err = coolfhir.ValidateTaskRequiredFields(task)
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("invalid Task: %w", err), "task validation failed")
	}

	var taskExisting fhir.Task
	exists := true
	fhirClient := s.fhirClientByTenant[request.Tenant.ID]
	if request.ResourceId == "" {
		// No ID, should be query parameters leading to the Task to update
		span.AddEvent("lookup_task_by_query_parameters")
		span.SetAttributes(attribute.String("fhir.task.lookup_method", "query"))

		if len(request.RequestUrl.Query()) == 0 {
			return nil, otel.Error(span, errors.New("missing Task ID or query parameters for selecting the Task to update"), "missing task id or query parameters")
		}
		var opts []fhirclient.Option
		for k, v := range request.RequestUrl.Query() {
			opts = append(opts, fhirclient.QueryParam(k, v[0]))
		}
		var resultBundle fhir.Bundle
		if err = fhirClient.Read("Task", &resultBundle, opts...); err != nil {
			return nil, otel.Error(span, fmt.Errorf("failed to search for Task to update: %w", err), "failed to search for task")
		}
		if len(resultBundle.Entry) == 0 {
			exists = false
		} else if len(resultBundle.Entry) > 1 {
			return nil, otel.Error(span, errors.New("multiple Tasks found to update, expected 1"), "multiple tasks found")
		} else {
			if err = coolfhir.ResourceInBundle(&resultBundle, coolfhir.EntryIsOfType("Task"), &taskExisting); err != nil {
				return nil, otel.Error(span, fmt.Errorf("failed to read Task from search result: %w", err), "failed to read task from search result")
			}
		}
		if task.Id != nil && *taskExisting.Id != *task.Id {
			return nil, otel.Error(span, coolfhir.BadRequest("ID in request URL does not match ID in resource"), "id mismatch")
		}
	} else {
		// Direct ID lookup
		span.AddEvent("lookup_task_by_id")
		span.SetAttributes(
			attribute.String("fhir.task.lookup_method", "id"),
			attribute.String(otel.FHIRTaskID, request.ResourceId),
		)

		if (task.Id != nil && request.ResourceId != "") && request.ResourceId != *task.Id {
			return nil, otel.Error(span, coolfhir.BadRequest("ID in request URL does not match ID in resource"), "id mismatch")
		}
		err = fhirClient.Read("Task/"+request.ResourceId, &taskExisting)
		// TODO: If the resource was identified by a concrete ID, and was intended as upsert (create-if-not-exists), this doesn't work yet.
	}
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("failed to read Task: %w", err), "failed to read task")
	}
	if !exists {
		// Doesn't exist, create it (upsert)
		span.AddEvent("upsert_task_creation")
		span.SetAttributes(attribute.String("fhir.task.operation_mode", "upsert_create"))
		return s.handleCreateTask(ctx, request, tx)
	}

	span.AddEvent("update_existing_task")
	span.SetAttributes(attribute.String("fhir.task.operation_mode", "update"))

	// Add existing task status for comparison
	if taskExisting.Status.String() != "" {
		span.SetAttributes(attribute.String("fhir.task.existing_status", taskExisting.Status.String()))
	}

	if task.Status != taskExisting.Status {
		// If the status is changing, validate the transition
		span.AddEvent("validating_status_transition")
		span.SetAttributes(attribute.Bool("fhir.task.status_changing", true))

		isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&taskExisting, request.Principal.Organization.Identifier)
		isScpSubTask := coolfhir.IsScpSubTask(&task)

		span.SetAttributes(
			attribute.Bool("fhir.task.is_owner", isOwner),
			attribute.Bool("fhir.task.is_requester", isRequester),
			attribute.Bool("fhir.task.is_scp_subtask", isScpSubTask),
		)

		if !isValidTransition(taskExisting.Status, task.Status, isOwner, isRequester, isScpSubTask) {
			return nil, otel.Error(span, fmt.Errorf("invalid state transition from %s to %s, owner(%t) requester(%t) scpSubtask(%t)",
				taskExisting.Status.String(),
				task.Status.String(),
				isOwner,
				isRequester,
				isScpSubTask), "invalid status transition")
		}
	} else {
		span.SetAttributes(attribute.Bool("fhir.task.status_changing", false))
	}

	// Check fields that aren't allowed to be changed: owner, requester, basedOn, partOf, for
	if !deep.Equal(task.Requester, taskExisting.Requester) {
		return nil, otel.Error(span, errors.New("Task.requester cannot be changed"), "task.requester cannot be changed")
	}
	if !deep.Equal(task.Owner, taskExisting.Owner) {
		return nil, otel.Error(span, errors.New("Task.owner cannot be changed"), "task.owner cannot be changed")
	}
	if !deep.Equal(task.BasedOn, taskExisting.BasedOn) {
		return nil, otel.Error(span, errors.New("Task.basedOn cannot be changed"), "task.basedOn cannot be changed")
	}
	if !deep.Equal(task.PartOf, taskExisting.PartOf) {
		return nil, otel.Error(span, errors.New("Task.partOf cannot be changed"), "task.partOf cannot be changed")
	}
	if !deep.Equal(task.For, taskExisting.For) {
		return nil, otel.Error(span, errors.New("Task.for cannot be changed"), "task.for cannot be changed")
	}

	// Resolve the CarePlan
	span.AddEvent("resolving_careplan_reference")
	carePlanRef, err := basedOn(task)
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("invalid Task.basedOn: %w", err), "invalid task.basedOn")
	}
	carePlanId := strings.TrimPrefix(*carePlanRef, "CarePlan/")
	span.SetAttributes(attribute.String("fhir.careplan.id", carePlanId))

	span.AddEvent("adding_task_update_to_transaction")
	idx := len(tx.Entry)
	taskBundleEntry := request.bundleEntryWithResource(task)
	tx = tx.AppendEntry(taskBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
	}))

	// Update care team
	span.AddEvent("updating_careteam")
	_, err = careteamservice.Update(ctx, fhirClient, carePlanId, task, request.LocalIdentity, tx)
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("update CareTeam: %w", err), "failed to update care team")
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(attribute.String("fhir.task.update", "success"))

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var updatedTask fhir.Task
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, fhirClient, request.BaseURL, &taskBundleEntry, &txResult.Entry[idx], &updatedTask)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original Task that was sent.
			updatedTask = task
		} else if err != nil {
			return nil, nil, err
		}
		var notifications = []any{&updatedTask}
		// If CareTeam was updated, notify about CareTeam
		var updatedCareTeam fhir.CareTeam
		if err := coolfhir.ResourceInBundle(txResult, coolfhir.EntryIsOfType("CareTeam"), &updatedCareTeam); err == nil {
			notifications = append(notifications, &updatedCareTeam)
		}
		return []*fhir.BundleEntry{result}, notifications, nil
	}, nil
}

func isValidTransition(from fhir.TaskStatus, to fhir.TaskStatus, isOwner bool, isRequester bool, isScpSubtask bool) bool {
	if isOwner == false && isRequester == false {
		return false
	}

	if isScpSubtask {
		return isOwner && from == fhir.TaskStatusReady && (to == fhir.TaskStatusCompleted || to == fhir.TaskStatusFailed)
	}

	// Transitions valid for owner only
	if isOwner {
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusReceived {
			return true
		}
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusAccepted {
			return true
		}
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusRejected {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusAccepted {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusRejected {
			return true
		}
		if from == fhir.TaskStatusAccepted && to == fhir.TaskStatusInProgress {
			return true
		}
		if from == fhir.TaskStatusAccepted && to == fhir.TaskStatusRejected {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusCompleted {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusFailed {
			return true
		}
		if from == fhir.TaskStatusReady && to == fhir.TaskStatusCompleted {
			return true
		}
		if from == fhir.TaskStatusReady && to == fhir.TaskStatusFailed {
			return true
		}
	}
	// Transitions valid for owner or requester
	if isOwner || isRequester {
		if from == fhir.TaskStatusRequested && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusReceived && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusAccepted && to == fhir.TaskStatusCancelled {
			return true
		}
		if from == fhir.TaskStatusInProgress && to == fhir.TaskStatusOnHold {
			return true
		}
		if from == fhir.TaskStatusOnHold && to == fhir.TaskStatusInProgress {
			return true
		}

	}
	return false
}
