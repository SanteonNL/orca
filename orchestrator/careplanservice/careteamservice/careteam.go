package careteamservice

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"slices"
	"strings"
	"time"
)

var nowFunc = time.Now

// Update updates the CareTeam for a CarePlan based on its activities.
// It implements the business rules specified by https://santeonnl.github.io/shared-care-planning/overview.html#creating-and-responding-to-a-task
// updateTrigger is the Task that triggered the update, which is used to determine the CareTeam membership.
// It's passed to the function, as the new Task is not yet stored in the FHIR server, since the update is to be done in a single transaction.
// When the CareTeam is updated, it adds the update(s) to the given transaction and returns true. If no changes are made, it returns false.
func Update(client fhirclient.Client, carePlanId string, updateTriggerTask fhir.Task, tx *coolfhir.BundleBuilder) (bool, error) {
	if len(updateTriggerTask.PartOf) > 0 {
		// Only update the CareTeam if the Task is not a subtask
		return false, nil
	}

	bundle := new(fhir.Bundle)
	if err := client.Read("CarePlan",
		bundle,
		fhirclient.QueryParam("_id", carePlanId),
		fhirclient.QueryParam("_include", "CarePlan:care-team"),
		fhirclient.QueryParam("_include", "CarePlan:activity-reference")); err != nil {
		return false, fmt.Errorf("unable to resolve CarePlan and related resources: %w", err)
	}

	carePlan := new(fhir.CarePlan)
	if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryHasID(carePlanId), carePlan); err != nil {
		return false, fmt.Errorf("CarePlan not found (id=%s): %w", carePlanId, err)
	}
	careTeam, err := resolveCareTeam(bundle, carePlan)
	if err != nil {
		return false, err
	}
	activities, err := resolveActivities(bundle, carePlan)
	if err != nil {
		return false, err
	}

	// TODO: ETag on CareTeam
	var otherActivities []fhir.Task
	for _, activity := range activities {
		if updateTriggerTask.Id == nil || *activity.Id != *updateTriggerTask.Id {
			otherActivities = append(otherActivities, activity)
		}
	}
	// Make sure Task.requester is always in the CareTeam
	// TODO: But are they always active, regardless of Task status?
	changed := ActivateMembership(careTeam, updateTriggerTask.Requester)
	if updateCareTeam(careTeam, otherActivities, updateTriggerTask) {
		changed = true
	}
	if changed {
		sortParticipants(careTeam.Participant)
		tx.Update(*careTeam, "CareTeam/"+*careTeam.Id)
		return true, nil
	}
	return false, nil
}

func updateCareTeam(careTeam *fhir.CareTeam, otherActivities []fhir.Task, updatedActivity fhir.Task) bool {
	if updatedActivity.Status == fhir.TaskStatusAccepted {
		// Task.owner should be an active member
		return ActivateMembership(careTeam, updatedActivity.Owner)
	}
	if updatedActivity.Status == fhir.TaskStatusCompleted ||
		updatedActivity.Status == fhir.TaskStatusFailed ||
		updatedActivity.Status == fhir.TaskStatusCancelled {
		// Task.owner should not be an active member, or should have an end date set if it was an active member.
		// If there's still other Tasks that are active (status accepted, in-progress or on-hold), the member should remain in the CareTeam.
		return deactivateMembership(careTeam, updatedActivity.Owner, otherActivities)
	}
	return false
}

func ActivateMembership(careTeam *fhir.CareTeam, party *fhir.Reference) bool {
	for _, participant := range careTeam.Participant {
		if coolfhir.IdentifierEquals(participant.Member.Identifier, party.Identifier) {
			// Already in CareTeam
			return false
		}
	}
	// Not yet in CareTeam, add member
	// TODO: Set Member
	careTeam.Participant = append(careTeam.Participant, fhir.CareTeamParticipant{
		Member: party,
		Period: &fhir.Period{
			Start: to.Ptr(now()),
		},
	})
	return true
}

func deactivateMembership(careTeam *fhir.CareTeam, party *fhir.Reference, otherActivities []fhir.Task) bool {
	// If the party has another Task that gives active membership, don't deactivate
	for _, activity := range otherActivities {
		if coolfhir.IdentifierEquals(activity.Owner.Identifier, party.Identifier) {
			// Still active
			return false
		}
	}

	var result bool
	for i, participant := range careTeam.Participant {
		if !coolfhir.IsLogicalReference(participant.Member) {
			continue
		}
		if coolfhir.IdentifierEquals(participant.Member.Identifier, party.Identifier) {
			// Update end date
			if participant.Period.End == nil {
				careTeam.Participant[i].Period.End = to.Ptr(now())
				result = true
			}
		}
	}
	return result
}

func resolveCareTeam(bundle *fhir.Bundle, carePlan *fhir.CarePlan) (*fhir.CareTeam, error) {
	if len(carePlan.CareTeam) != 1 {
		return nil, errors.New("CarePlan must have exactly one CareTeam")
	}
	var currentCareTeam fhir.CareTeam
	if len(carePlan.CareTeam) == 1 {
		// prevent nil deref
		ref := carePlan.CareTeam[0].Reference
		if ref == nil {
			return nil, errors.New("CarePlan.CareTeam.Reference is required")
		}
		// Should be resolvable in Bundle
		if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryHasID(*ref), &currentCareTeam); err != nil {
			return nil, fmt.Errorf("unable to resolve CarePlan.CareTeam: %w", err)
		}
	}
	return &currentCareTeam, nil
}

func resolveActivities(bundle *fhir.Bundle, carePlan *fhir.CarePlan) ([]fhir.Task, error) {
	var activityRefs []string
	for _, activityRef := range carePlan.Activity {
		if activityRef.Reference == nil || activityRef.Reference.Reference == nil {
			return nil, errors.New("CarePlan.Activity all must be a FHIR Reference with a string reference")
		}
		if activityRef.Reference.Type == nil || *activityRef.Reference.Type != "Task" {
			return nil, errors.New("CarePlan.Activity.Reference must be of type Task")
		}
		activityRefs = append(activityRefs, *activityRef.Reference.Reference)
	}
	var tasks []fhir.Task
	for _, ref := range activityRefs {
		if err := coolfhir.ResourcesInBundle(bundle, coolfhir.FilterResource(func(resource coolfhir.Resource) bool {
			return resource.Type == "Task" && "Task/"+resource.ID == ref
		}), &tasks); err != nil {
			return nil, fmt.Errorf("unable to resolve Task in bundle (id=%s): %w", ref, err)
		}
	}
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
