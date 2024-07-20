package careteamservice

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"time"
)

func Update2(client fhirclient.Client, carePlanId string,) ([]fhir.BundleEntry, error) {

// Update updates the CareTeam for a CarePlan based on its activities.
// It implements the business rules specified by https://santeonnl.github.io/shared-care-planning/overview.html#creating-and-responding-to-a-task
func Update(client fhirclient.Client, carePlanId string) ([]fhir.BundleEntry, error) {
	var bundle fhir.Bundle
	if err := client.Read("CarePlan",
		&bundle,
		fhirclient.QueryParam("_id", carePlanId),
		fhirclient.QueryParam("_include", "CarePlan:care-team"),
		fhirclient.QueryParam("_include", "CarePlan:activity-reference")); err != nil {
		return nil, fmt.Errorf("unable to resolve CarePlan and related resources: %w", err)
	}

	carePlan, err := resolveCarePlan(bundle, carePlanId)
	if err != nil {
		return nil, err
	}
	careTeam, err := coolfhir.ResourceInBundle(bundle, carePlan., &activity); err != nil {
		return nil, fmt.Errorf("unable to resolve CarePlan.Activity.Reference (ref=%s): %w", ref, err)
	}
	tasks, err := resolveTasks(bundle, carePlan)
	if err != nil {
		return nil, err
	}

	updateCareTeam()

	return nil, nil
}

func updateCareTeam(careTeam *fhir.CareTeam, activities []fhir.Task) (bool, error) {

	// Add new/changed memberships
	var changed bool
	targetMembers := collectMemberships(activities)
outer:
	for member, details := range targetMembers {
		for i, curr := range careTeam.Participant {
			if !coolfhir.IsLogicalReference(curr.Member) {
				continue
			}
			if *curr.Member.Identifier.System == member.System && *curr.Member.Identifier.Value == member.Value {
				// Already in CareTeam
				if details.Ended && curr.Period == nil {
					// Set DateEnded
					curr.Period = &fhir.Period{
						End: to.Ptr(now()),
					}
					careTeam.Participant[i] = curr
					changed = true
				}
				// TODO: Might have to set DateEnded
				continue outer
			}
		}
		// Not yet in CareTeam, add member
		careTeam.Participant = append(careTeam.Participant, fhir.CareTeamParticipant{
			Member: &fhir.Reference{
				Type: to.Ptr(coolfhir.TypeOrganization),
				Identifier: &fhir.Identifier{
					System: &member.System,
					Value:  &member.Value,
				},
			},
			Period: &fhir.Period{
				Start: to.Ptr(now()),
			},
		})
	}
	// TODO: Remove members that don't have a Task? E.g., someone added to CareTeam manually?
	return nil, nil
}

func resolveCarePlan(bundle fhir.Bundle, carePlanId string) (*fhir.CarePlan, error) {
	var carePlan fhir.CarePlan
	if err := coolfhir.ResourceInBundle(bundle, carePlanId, &carePlan); err != nil {
		return nil, fmt.Errorf("CarePlan not found (id=%s): %w", carePlanId, err)
	}

	// Resolve CareTeam
	if len(carePlan.CareTeam) > 1 {
		return nil, errors.New("CarePlan can't have multiple CareTeams")
	}
	var currentCareTeam fhir.CareTeam
	if len(carePlan.CareTeam) == 1 {
		// prevent nil deref
		if carePlan.CareTeam[0].Id == nil {
			return nil, errors.New("CarePlan.CareTeam.Id is required")
		}
		// Should be resolvable in Bundle
		if err := coolfhir.ResourceInBundle(bundle, *carePlan.CareTeam[0].Id, &currentCareTeam); err != nil {
			return nil, fmt.Errorf("unable to resolve CarePlan.CareTeam: %w", err)
		}
	}
	return &carePlan, nil
}

func resolveCareTeam(bundle fhir.Bundle, carePlan *fhir.CarePlan) (*fhir.CareTeam, error) {
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
		if err := coolfhir.ResourceInBundle(bundle, *carePlan.CareTeam[0].Id, &currentCareTeam); err != nil {
			return nil, fmt.Errorf("unable to resolve CarePlan.CareTeam: %w", err)
		}
	}
	return &currentCareTeam, nil
}

func resolveTasks(bundle fhir.Bundle, carePlan *fhir.CarePlan) ([]fhir.Task, error) {
	// Resolve activities
	var activities []fhir.Task
	for _, activityRef := range carePlan.Activity {
		if activityRef.Reference == nil || activityRef.Reference.Reference == nil {
			return nil, errors.New("CarePlan.Activity all must be a reference with an reference")
		}
		if activityRef.Reference.Type == nil || *activityRef.Reference.Type != "Task" {
			return nil, errors.New("CarePlan.Activity.Reference must be of type Task")
		}
		var activity fhir.Task
		ref := *activityRef.Reference.Reference
		if err := coolfhir.ResourceInBundle(bundle, ref, &activity); err != nil {
			return nil, fmt.Errorf("unable to resolve CarePlan.Activity.Reference (ref=%s): %w", ref, err)
		}
		activities = append(activities, activity)
	}
	return activities, nil
}

type memberID struct {
	System string
	Value  string
}

type membership struct {
	// Ended indicates the participant's membership has ended, causing endDate to be set
	Ended bool
}

func collectMemberships(activities []fhir.Task) map[memberID]membership {
	result := make(map[memberID]membership)
	for _, activity := range activities {
		// Add Task.requester to CareTeam
		if err := coolfhir.ValidateLogicalReference(activity.Requester, coolfhir.TypeOrganization, coolfhir.URANamingSystem); err != nil {
			log.Info().Msgf("Task.requester is invalid, skipping (id=%s): %v", to.Value(activity.Id), err)
		}
		result[memberID{System: "Organization", Value: *activity.Requester.Identifier.Value}] = membership{}
		// Add Task.owner to CareTeam
		if err := coolfhir.ValidateLogicalReference(activity.Owner, coolfhir.TypeOrganization, coolfhir.URANamingSystem); err != nil {
			log.Info().Msgf("Task.owner is invalid, skipping (id=%s): %v", to.Value(activity.Id), err)
		}
		if taskOwnerIsMember(activity.Status) {
			p := memberID{System: "Organization", Value: *activity.Owner.Identifier.Value}
			// Check existence, otherwise we could accidentally override ended==false with true,
			// while an active Task always give membership (over any cancelled Tasks).
			if _, exists := result[p]; !exists {
				result[p] = membership{
					Ended: activity.Status == fhir.TaskStatusFailed, // If Task.status=failed, set end date
				}
			}
		}
	}
	return result
}

func taskOwnerIsMember(status fhir.TaskStatus) bool {
	switch status {
	case fhir.TaskStatusAccepted:
		fallthrough
	case fhir.TaskStatusInProgress:
		fallthrough
	case fhir.TaskStatusCompleted:
		fallthrough
	case fhir.TaskStatusFailed:
		fallthrough
	case fhir.TaskStatusOnHold:
		return true
	default:
		return false
	}
}

func now() string {
	return time.Now().Format(time.RFC3339)
}
