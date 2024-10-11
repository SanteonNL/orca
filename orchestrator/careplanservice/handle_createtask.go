package careplanservice

import (
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/google/uuid"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/careteamservice"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateTask(httpRequest *http.Request, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	log.Info().Msg("Creating Task")
	var task fhir.Task
	if err := s.readRequest(httpRequest, &task); err != nil {
		return nil, fmt.Errorf("invalid Task: %w", err)
	}

	switch task.Status {
	case fhir.TaskStatusRequested:
	case fhir.TaskStatusReady:
	default:
		return nil, errors.New(fmt.Sprintf("cannot create Task with status %s, must be %s or %s", task.Status, fhir.TaskStatusRequested.String(), fhir.TaskStatusReady.String()))
	}

	principal, err := auth.PrincipalFromContext(httpRequest.Context())
	if err != nil {
		return nil, err
	}

	carePlan := fhir.CarePlan{}
	var carePlanRef *string
	err = coolfhir.ValidateTaskRequiredFields(task)
	if err != nil {
		return nil, err
	}
	// Resolve the CarePlan
	if len(task.BasedOn) == 0 {
		// The CarePlan does not exist, a CarePlan and CareTeam will be created and the requester will be added as a member
		// Create a new CarePlan (which will also create a new CareTeam) based on the Task reference
		carePlanURL := "urn:uuid:" + uuid.NewString()
		careTeamURL := "urn:uuid:" + uuid.NewString()
		taskURL := "urn:uuid:" + uuid.NewString()
		careTeam := fhir.CareTeam{}
		carePlan.CareTeam = append(carePlan.CareTeam, fhir.Reference{
			Reference: to.Ptr(careTeamURL),
			Type:      to.Ptr(coolfhir.ResourceType(careTeam)),
		})

		carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
			Reference: &fhir.Reference{
				Reference: to.Ptr(taskURL),
				Type:      to.Ptr(coolfhir.ResourceType(task)),
			},
		})
		task.BasedOn = []fhir.Reference{
			{
				Type:      to.Ptr(coolfhir.ResourceType(carePlan)),
				Reference: to.Ptr(carePlanURL),
			},
		}

		ok := careteamservice.ActivateMembership(&careTeam, &fhir.Reference{
			Identifier: &principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
		if !ok {
			return nil, errors.New("failed to activate membership for new CareTeam")
		}

		tx.Create(carePlan, coolfhir.WithFullUrl(carePlanURL)).
			Create(careTeam, coolfhir.WithFullUrl(careTeamURL)).
			Create(task, coolfhir.WithFullUrl(taskURL))
	} else {
		// Adding a task to an existing CarePlan
		carePlanRef, err = basedOn(task)
		if err != nil {
			return nil, fmt.Errorf("invalid Task.basedOn: %w", err)
		}
		// we have a valid reference to a CarePlan, use this to retrieve the CarePlan and CareTeam to validate the requester is a participant
		var careTeams []fhir.CareTeam
		err = s.fhirClient.Read(*carePlanRef, &carePlan, fhirclient.ResolveRef("careTeam", &careTeams))
		if err != nil {
			return nil, err
		}
		if len(careTeams) == 0 {
			return nil, coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
		}

		participant := coolfhir.FindMatchingParticipantInCareTeam(careTeams, principal.Organization.Identifier)
		if participant == nil {
			return nil, coolfhir.NewErrorWithCode("requester is not part of CareTeam", http.StatusUnauthorized)
		}
		// TODO: Manage time-outs properly
		// Add Task to CarePlan.activities
		err = s.newTaskInExistingCarePlan(tx, task, &carePlan)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create Task: %w", err)
	}
	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdTask fhir.Task
		result, err := coolfhir.FetchBundleEntry(s.fhirClient, txResult, func(entry fhir.BundleEntry) bool {
			return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/")
		}, &createdTask)
		if err != nil {
			return nil, nil, err
		}
		var notifications = []any{&createdTask}
		// If CareTeam was updated, notify about CareTeam
		var updatedCareTeam fhir.CareTeam
		if err := coolfhir.ResourceInBundle(txResult, coolfhir.EntryIsOfType("CareTeam"), &updatedCareTeam); err == nil {
			notifications = append(notifications, &updatedCareTeam)
		}

		// TODO: this really shouldn't be here. Maybe move to CPC and call it in-process (e.g. goroutine?)
		// TODO: This should be done by the CPC after the notification is received
		if err = s.handleTaskFillerCreate(&createdTask); err != nil {
			return nil, nil, fmt.Errorf("failed to handle TaskFillerCreate: %w", err)
		}
		return result, []any{&createdTask}, nil
	}, nil
}

// newTaskInExistingCarePlan creates a new Task and references the Task from the CarePlan.activities.
func (s *Service) newTaskInExistingCarePlan(tx *coolfhir.TransactionBuilder, task fhir.Task, carePlan *fhir.CarePlan) error {
	taskRef := "urn:uuid:" + uuid.NewString()
	carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
		Reference: &fhir.Reference{
			Reference: to.Ptr(taskRef),
			Type:      to.Ptr("Task"),
		},
	})

	// TODO: Only if not updated
	tx.Create(task, coolfhir.WithFullUrl(taskRef)).
		Update(*carePlan, "CarePlan/"+*carePlan.Id)

	if _, err := careteamservice.Update(s.fhirClient, *carePlan.Id, task, tx); err != nil {
		return fmt.Errorf("failed to update CarePlan: %w", err)
	}
	return nil
}

// basedOn returns the CarePlan reference the Task is based on, e.g. CarePlan/123.
func basedOn(task fhir.Task) (*string, error) {
	if len(task.BasedOn) != 1 {
		return nil, errors.New("Task.basedOn must have exactly one reference")
	} else if task.BasedOn[0].Reference == nil || !strings.HasPrefix(*task.BasedOn[0].Reference, "CarePlan/") {
		return nil, errors.New("Task.basedOn must contain a relative reference to a CarePlan")
	}
	return task.BasedOn[0].Reference, nil
}
