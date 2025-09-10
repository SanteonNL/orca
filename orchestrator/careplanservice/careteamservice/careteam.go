package careteamservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"slices"
	"strings"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	nowFunc = time.Now
	tracer  = baseotel.Tracer("careplanservice.careteamservice")
)

// Update updates the CareTeam for a CarePlan based on its activities.
// It implements the business rules specified by https://santeonnl.github.io/shared-care-planning/overview.html#creating-and-responding-to-a-task
// updateTrigger is the Task that triggered the update, which is used to determine the CareTeam membership.
// It's passed to the function, as the new Task is not yet stored in the FHIR server, since the update is to be done in a single transaction.
// When the CareTeam is updated, it adds the update(s) to the given transaction and returns true. If no changes are made, it returns false.
func Update(ctx context.Context, client fhirclient.Client, carePlanId string, updateTriggerTask fhir.Task, localIdentity *fhir.Identifier, tx *coolfhir.BundleBuilder) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithAttributes(
			attribute.String("careplan_id", carePlanId),
			attribute.String("task_id", to.Value(updateTriggerTask.Id)),
			attribute.String("task_status", updateTriggerTask.Status.String()),
		),
	)
	defer span.End()

	if len(updateTriggerTask.PartOf) > 0 {
		span.AddEvent("skipping_subtask")
		return false, nil
	}

	bundle := new(fhir.Bundle)
	span.AddEvent("fetching_careplan_and_activities")
	if err := client.Read("CarePlan",
		bundle,
		fhirclient.QueryParam("_id", carePlanId),
		fhirclient.QueryParam("_include", "CarePlan:activity-reference")); err != nil {
		span.RecordError(err)
		return false, otel.Error(span, fmt.Errorf("unable to resolve CarePlan and related resources: %w", err))
	}

	carePlan := new(fhir.CarePlan)
	if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryHasID(carePlanId), carePlan); err != nil {
		return false, otel.Error(span, fmt.Errorf("CarePlan not found (id=%s): %w", carePlanId, err))
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(carePlan)
	if err != nil {
		return false, otel.Error(span, err)
	}

	activities, err := resolveActivities(ctx, bundle, carePlan)
	if err != nil {
		return false, otel.Error(span, err)
	}

	span.SetAttributes(attribute.Int("activities_count", len(activities)))

	// TODO: ETag on CareTeam
	var otherActivities []fhir.Task
	for _, activity := range activities {
		if updateTriggerTask.Id == nil || *activity.Id != *updateTriggerTask.Id {
			otherActivities = append(otherActivities, activity)
		}
	}

	// Make sure Task.requester is always in the CareTeam
	// TODO: But are they always active, regardless of Task status?
	changed := ActivateMembership(ctx, careTeam, updateTriggerTask.Requester)
	if updateCareTeam(ctx, careTeam, otherActivities, updateTriggerTask) {
		changed = true
	}

	span.SetAttributes(attribute.Bool("careteam_changed", changed))

	if changed {
		span.AddEvent("updating_careteam")
		sortParticipants(careTeam.Participant)

		contained, err := coolfhir.UpdateContainedResource(carePlan.Contained, &carePlan.CareTeam[0], careTeam)
		if err != nil {
			return false, otel.Error(span, fmt.Errorf("unable to update CarePlan.Contained: %w", err))
		}

		carePlan.Contained = contained
		tx.Update(carePlan, "CarePlan/"+*carePlan.Id, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: updateTriggerTask.Requester,
			Observer:    *localIdentity,
			Action:      fhir.AuditEventActionU,
		}))

		return true, nil
	}

	return false, nil
}

func updateCareTeam(ctx context.Context, careTeam *fhir.CareTeam, otherActivities []fhir.Task, updatedActivity fhir.Task) bool {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithAttributes(
			attribute.String("task_status", updatedActivity.Status.String()),
			attribute.Int("other_activities_count", len(otherActivities)),
		),
	)
	defer span.End()

	if updatedActivity.Status == fhir.TaskStatusAccepted {
		return ActivateMembership(ctx, careTeam, updatedActivity.Owner)
	}
	if updatedActivity.Status == fhir.TaskStatusCompleted ||
		updatedActivity.Status == fhir.TaskStatusFailed ||
		updatedActivity.Status == fhir.TaskStatusCancelled {
		return deactivateMembership(ctx, careTeam, updatedActivity.Owner, otherActivities)
	}
	return false
}

func ActivateMembership(ctx context.Context, careTeam *fhir.CareTeam, party *fhir.Reference) bool {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithAttributes(
			attribute.String("party_identifier", to.Value(party.Identifier.Value)),
		),
	)
	defer span.End()

	span.AddEvent("activating_membership")

	for _, participant := range careTeam.Participant {
		if coolfhir.IdentifierEquals(participant.Member.Identifier, party.Identifier) {
			span.AddEvent("member_already_in_careteam")
			return false
		}
	}

	span.AddEvent("adding_member_to_careteam")
	careTeam.Participant = append(careTeam.Participant, fhir.CareTeamParticipant{
		Member: party,
		Period: &fhir.Period{
			Start: to.Ptr(now()),
		},
	})
	return true
}

func deactivateMembership(ctx context.Context, careTeam *fhir.CareTeam, party *fhir.Reference, otherActivities []fhir.Task) bool {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithAttributes(
			attribute.String("party_identifier", to.Value(party.Identifier.Value)),
			attribute.Int("other_activities_count", len(otherActivities)),
		),
	)
	defer span.End()

	span.AddEvent("deactivating_membership")

	// If the party has another Task that gives active membership, don't deactivate
	for _, activity := range otherActivities {
		if coolfhir.IdentifierEquals(activity.Owner.Identifier, party.Identifier) {
			span.AddEvent("member_still_active_in_other_tasks")
			return false
		}
	}

	var result bool
	for i, participant := range careTeam.Participant {
		if !coolfhir.IsLogicalReference(participant.Member) {
			continue
		}
		if coolfhir.IdentifierEquals(participant.Member.Identifier, party.Identifier) {
			if participant.Period.End == nil {
				span.AddEvent("setting_end_date_for_member")
				careTeam.Participant[i].Period.End = to.Ptr(now())
				result = true
			}
		}
	}

	span.SetAttributes(attribute.Bool("membership_deactivated", result))
	return result
}

func resolveActivities(ctx context.Context, bundle *fhir.Bundle, carePlan *fhir.CarePlan) ([]fhir.Task, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithAttributes(
			attribute.Int("activity_references_count", len(carePlan.Activity)),
		),
	)
	defer span.End()

	var activityRefs []string
	for _, activityRef := range carePlan.Activity {
		if activityRef.Reference == nil || activityRef.Reference.Reference == nil {
			err := errors.New("CarePlan.Activity all must be a FHIR Reference with a string reference")
			span.RecordError(err)
			return nil, err
		}
		if activityRef.Reference.Type == nil || *activityRef.Reference.Type != "Task" {
			err := errors.New("CarePlan.Activity.Reference must be of type Task")
			span.RecordError(err)
			return nil, err
		}
		activityRefs = append(activityRefs, *activityRef.Reference.Reference)
	}

	var tasks []fhir.Task
	for _, ref := range activityRefs {
		if err := coolfhir.ResourcesInBundle(bundle, coolfhir.FilterResource(func(resource coolfhir.Resource) bool {
			return resource.Type == "Task" && "Task/"+resource.ID == ref
		}), &tasks); err != nil {
			span.RecordError(err)
			return nil, fmt.Errorf("unable to resolve Task in bundle (id=%s): %w", ref, err)
		}
	}

	span.SetAttributes(attribute.Int("resolved_tasks_count", len(tasks)))
	return tasks, nil
}

func now() string {
	return nowFunc().Format(time.RFC3339)
}

func sortParticipants(participants []fhir.CareTeamParticipant) {
	// Sort by Member, so that the order doesn't change every time the CareTeam is updated
	// This also aids in testing, as the order of participants is predictable.
	slices.SortFunc(participants, func(a, b fhir.CareTeamParticipant) int {
		return strings.Compare(to.Value(a.Member.Identifier.Value), to.Value(b.Member.Identifier.Value))
	})
}
