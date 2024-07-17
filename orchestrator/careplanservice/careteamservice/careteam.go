package careteamservice

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
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
func Update(client fhirclient.Client, carePlanId string, updateTriggerTask fhir.Task, tx *coolfhir.TransactionBuilder) (bool, error) {
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
	// Make sure the CareTeam is updated given the new Task state
	var taskUpdated bool
	for i, task := range activities {
		if task.Id != nil && updateTriggerTask.Id != nil && *task.Id == *updateTriggerTask.Id {
			activities[i] = updateTriggerTask
			taskUpdated = true
			break
		}
	}
	if !taskUpdated {
		// New task, add to activities
		activities = append(activities, updateTriggerTask)
	}

	// TODO: ETag on CareTeam
	changed, err := updateCareTeam(careTeam, activities)
	if changed {
		sortParticipants(careTeam.Participant)
		tx.Update(*careTeam, "CareTeam/"+*careTeam.Id)
		return true, nil
	}
	return false, nil
}

func updateCareTeam(careTeam *fhir.CareTeam, activities []fhir.Task) (bool, error) {
	// Add new/changed memberships
	var changed bool
	targetParticipants := collectParticipants(activities)
outer:
	for targetParticipant, details := range targetParticipants {
		for i, curr := range careTeam.Participant {
			if !coolfhir.IsLogicalReference(curr.OnBehalfOf) {
				continue
			}
			if *curr.OnBehalfOf.Identifier.System == targetParticipant.System && *curr.OnBehalfOf.Identifier.Value == targetParticipant.Value {
				// Already in CareTeam
				if details.Ended {
					// Set DateEnded
					curr.Period.End = to.Ptr(now())
					careTeam.Participant[i] = curr
					changed = true
				}
				// TODO: Might have to set DateEnded
				continue outer
			}
		}
		// Not yet in CareTeam, add member
		careTeam.Participant = append(careTeam.Participant, fhir.CareTeamParticipant{
			OnBehalfOf: &fhir.Reference{
				Type: to.Ptr(coolfhir.TypeOrganization),
				Identifier: &fhir.Identifier{
					System: &targetParticipant.System,
					Value:  &targetParticipant.Value,
				},
			},
			Period: &fhir.Period{
				Start: to.Ptr(now()),
			},
		})
		changed = true
	}
	// TODO: Remove members that don't have a Task? E.g., someone added to CareTeam manually?
	return changed, nil
}

func resolveCareTeam(bundle *fhir.Bundle, carePlan *fhir.CarePlan) (*fhir.CareTeam, error) {
	if len(carePlan.CareTeam) != 1 {
		return nil, errors.New("CarePlan must have exactly one CareTeam")
	}
	var currentCareTeam fhir.CareTeam
	if len(carePlan.CareTeam) == 1 {
		// prevent nil deref
		if carePlan.CareTeam[0].Id == nil {
			return nil, errors.New("CarePlan.CareTeam.Id is required")
		}
		// Should be resolvable in Bundle
		if err := coolfhir.ResourceInBundle(bundle, coolfhir.EntryIsOfType("CareTeam"), &currentCareTeam); err != nil {
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

type participantID struct {
	System string
	Value  string
}

func (m participantID) String() string {
	return fmt.Sprintf("%s|%s", m.System, m.Value)
}

type participantDetails struct {
	// Ended indicates the participant's membership has ended, causing endDate to be set
	Ended bool
}

func collectParticipants(activities []fhir.Task) map[participantID]participantDetails {
	result := make(map[participantID]participantDetails)
	for _, activity := range activities {
		// Add Task.requester to CareTeam
		if err := coolfhir.ValidateLogicalReference(activity.Requester, coolfhir.TypeOrganization, coolfhir.URANamingSystem); err != nil {
			log.Warn().Msgf("Task.requester is invalid, skipping (id=%s): %v", to.Value(activity.Id), err)
			continue
		}
		result[participantID{System: *activity.Requester.Identifier.System, Value: *activity.Requester.Identifier.Value}] = participantDetails{}
		// Add Task.owner to CareTeam
		if err := coolfhir.ValidateLogicalReference(activity.Owner, coolfhir.TypeOrganization, coolfhir.URANamingSystem); err != nil {
			log.Warn().Msgf("Task.owner is invalid, skipping (id=%s): %v", to.Value(activity.Id), err)
			continue
		}
		if taskOwnerIsMember(activity.Status) {
			p := participantID{System: *activity.Owner.Identifier.System, Value: *activity.Owner.Identifier.Value}
			// Check existence, otherwise we could accidentally override ended==false with true,
			// while an active Task always give membership (over any cancelled Tasks).
			if _, exists := result[p]; !exists {
				result[p] = participantDetails{
					// If Task.status=failed||completed, set end date
					Ended: activity.Status == fhir.TaskStatusFailed ||
						activity.Status == fhir.TaskStatusCompleted,
				}
			}
		}
	}
	return result
}

// taskOwnerIsMember returns true if the Task's owner is a member of the CareTeam.
// It implements the table as specified in https://build.fhir.org/ig/santeonnl/shared-care-planning/branches/main/overview.html#updating-careplan-and-careteam
func taskOwnerIsMember(status fhir.TaskStatus) bool {
	switch status {
	case fhir.TaskStatusAccepted:
		fallthrough
	case fhir.TaskStatusInProgress:
		fallthrough
	case fhir.TaskStatusOnHold:
		fallthrough
	case fhir.TaskStatusCompleted:
		fallthrough
	case fhir.TaskStatusFailed:
		return true
	default:
		return false
	}
}

func now() string {
	return nowFunc().Format(time.RFC3339)
}

func sortParticipants(participants []fhir.CareTeamParticipant) {
	// Sort by OnBehalfOf, so that the order doesn't change every time the CareTeam is updated
	// This also aids in testing, as the order of participants is predictable.
	slices.SortFunc(participants, func(a, b fhir.CareTeamParticipant) int {
		return strings.Compare(to.Value(a.OnBehalfOf.Identifier.Value), to.Value(b.OnBehalfOf.Identifier.Value))
	})
}
