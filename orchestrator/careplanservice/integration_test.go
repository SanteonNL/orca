package careplanservice

import (
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

var notificationCounter = new(atomic.Int32)

func Test_Integration_TaskLifecycle(t *testing.T) {
	// Note: this test consists of multiple steps that look like subtests, but they can't be subtests:
	//       in Golang, running a single Subtest causes the other tests not to run.
	//       This causes issues, since each test step (e.g. accepting Task) requires the previous step (test) to succeed (e.g. creating Task).
	t.Log("This test requires creates a new CarePlan and Task, then runs the Task through requested->accepted->completed lifecycle.")
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanContributor1, carePlanContributor2, invalidCarePlanContributor := setupIntegrationTest(t, notificationEndpoint)

	t.Run("Example bundle 1", func(t *testing.T) {
		t.Skip("TODO")
		bundleData, err := os.ReadFile("testdata/bundles/testbundle-1.json")
		require.NoError(t, err)
		testBundle(t, carePlanContributor1, bundleData)
	})

	notificationCounter.Store(0)

	participant1 := fhir.CareTeamParticipant{
		Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
		Period: &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2 := fhir.CareTeamParticipant{
		Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period: &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2WithEndDate := fhir.CareTeamParticipant{
		Member: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period: &fhir.Period{
			Start: to.Ptr("2021-01-01T00:00:00Z"),
			End:   to.Ptr("2021-01-02T00:00:00Z"),
		},
	}

	// Create patient, this will be used as the subject of the CarePlan
	patient := fhir.Patient{
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	err := carePlanContributor1.Create(patient, &patient)
	require.NoError(t, err)

	// Patient not associated with any CarePlans or CareTeams for negative auth testing
	var patient2 fhir.Patient
	err = carePlanContributor1.Create(fhir.Patient{
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("12345"),
			},
		},
	}, &patient2)
	require.NoError(t, err)

	var carePlan fhir.CarePlan
	var primaryTask fhir.Task
	t.Log("Creating Task - CarePlan does not exist")
	{
		primaryTask = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/123"),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - No BasedOn, requester is not care organization so creation fails")
	{
		primaryTask = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor2.Create(primaryTask, &primaryTask)
		require.Error(t, err)
	}

	t.Log("Creating Task - No BasedOn, no Task.For so primaryTask creation fails")
	{
		primaryTask = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.Error(t, err)
	}

	t.Log("Creating Task - Task is created through upsert (PUT on non-existing resource)")
	{
		primaryTask = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
				//Reference: to.Ptr("Patient/" + *patient.Id),
			},
		}

		err := carePlanContributor1.Update("Task", primaryTask, &primaryTask, fhirclient.QueryParam("_id", "123"))
		require.NoError(t, err)
		notificationCounter.Store(0)
		// Resolve created CarePlan through Task.basedOn
		require.NoError(t, carePlanContributor1.Read(*primaryTask.BasedOn[0].Reference, &carePlan))
	}

	t.Log("Create Subtask")
	{
		subTask := fhir.Task{
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: primaryTask.Owner,
			Owner:     primaryTask.Requester,
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			PartOf: []fhir.Reference{
				{
					Type:      to.Ptr("Task"),
					Reference: to.Ptr("Task/" + *primaryTask.Id),
				},
			},
		}
		err := carePlanContributor2.Create(subTask, &subTask)
		require.NoError(t, err)
		notificationCounter.Store(0)
		// INT-440: CarePlan.activity should not contain subtasks
		require.NoError(t, carePlanContributor1.Read("CarePlan/"+*carePlan.Id, &carePlan))
		require.Len(t, carePlan.Activity, 1)
	}

	t.Log("Creating Task - No BasedOn, new CarePlan and CareTeam are created")
	{
		primaryTask = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
				//Reference: to.Ptr("Patient/" + *patient.Id),
			},
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.NoError(t, err)
		err = carePlanContributor1.Read(*primaryTask.BasedOn[0].Reference, &carePlan)
		require.NoError(t, err)

		t.Run("Check CarePlan properties", func(t *testing.T) {
			require.Equal(t, fhir.CarePlanIntentOrder, carePlan.Intent)
			require.Equal(t, fhir.RequestStatusActive, carePlan.Status)
			require.Equal(t, *primaryTask.For, carePlan.Subject)
		})
		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, primaryTask.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *primaryTask.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 1)
			require.Equal(t, "Task", *carePlan.Activity[0].Reference.Type)
			require.Equal(t, "Task/"+*primaryTask.Id, *carePlan.Activity[0].Reference.Reference)
		})
		t.Run("Check that CareTeam now contains the requesting party", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Equal(t, 2, int(notificationCounter.Load()))
			notificationCounter.Store(0)
		})
	}

	t.Log("Search CarePlan")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Read("CarePlan", &searchResult, fhirclient.QueryParam("_id", *carePlan.Id), fhirclient.QueryParam("_include", "CarePlan:care-team"))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 2, "Expected 1 CarePlan and 1 CareTeam")
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "CarePlan/"+*carePlan.Id))
		require.True(t, strings.HasSuffix(*searchResult.Entry[1].FullUrl, *carePlan.CareTeam[0].Reference))
	}

	t.Log("Read CarePlan - Not in participants")
	{
		var fetchedCarePlan fhir.CarePlan
		err := invalidCarePlanContributor.Read("CarePlan/"+*carePlan.Id, &fetchedCarePlan)
		require.Error(t, err)
	}
	t.Log("Read CareTeam")
	{
		var fetchedCareTeam fhir.CareTeam
		err := carePlanContributor1.Read(*carePlan.CareTeam[0].Reference, &fetchedCareTeam)
		require.NoError(t, err)
		assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
	}
	t.Log("Read CareTeam - Does not exist")
	{
		var fetchedCareTeam fhir.CareTeam
		err := carePlanContributor1.Read("CarePlan/999", &fetchedCareTeam)
		require.Error(t, err)
	}
	t.Log("Read CareTeam - Not in participants")
	{
		var fetchedCareTeam fhir.CareTeam
		err := invalidCarePlanContributor.Read(*carePlan.CareTeam[0].Reference, &fetchedCareTeam)
		require.Error(t, err)
	}
	t.Log("Read Task")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor1.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.NoError(t, err)
		require.NotNil(t, fetchedTask.Id)
		require.Equal(t, fhir.TaskStatusRequested, fetchedTask.Status)
		assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
	}
	t.Log("Read Task - Non-creating referenced party")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor2.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.NoError(t, err)
		require.NotNil(t, fetchedTask.Id)
		require.Equal(t, fhir.TaskStatusRequested, fetchedTask.Status)
	}
	t.Log("Read Task - Not in participants")
	{
		var fetchedTask fhir.Task
		err := invalidCarePlanContributor.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Read Task - Does not exist")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor1.Read("Task/999", &fetchedTask)
		require.Error(t, err)
	}
	previousTask := primaryTask

	t.Log("Creating Task - Invalid status Accepted")
	{
		primaryTask = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusAccepted,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - Invalid status Draft")
	{
		primaryTask = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusDraft,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - Existing CarePlan")
	{
		primaryTask = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.NoError(t, err)
		err = carePlanContributor1.Read("CarePlan/"+*carePlan.Id, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, primaryTask.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *primaryTask.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 2)
			for _, activity := range carePlan.Activity {
				require.Equal(t, "Task", *activity.Reference.Type)
				require.Equal(t, true, "Task/"+*primaryTask.Id == *activity.Reference.Reference || "Task/"+*previousTask.Id == *activity.Reference.Reference)
			}
		})
		t.Run("Check that CareTeam now contains the requesting party", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Equal(t, 2, int(notificationCounter.Load()))
			notificationCounter.Store(0)
		})
	}

	t.Log("Care Team Search")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Read("CareTeam", &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 2, "Expected 1 team")
	}

	t.Log("Accepting Task")
	{
		primaryTask.Status = fhir.TaskStatusAccepted
		var updatedTask fhir.Task
		// Note: use FHIR search instead of specifying ID to test support for updating resources identified by logical identifiers
		err := carePlanContributor2.Update("Task", primaryTask, &updatedTask, fhirclient.QueryParam("_id", *primaryTask.Id))
		//err := carePlanContributor2.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.NoError(t, err)
		primaryTask = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusAccepted, updatedTask.Status)
		})
		t.Run("Check that CareTeam now contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor2, *carePlan.CareTeam[0].Reference, participant1, participant2)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Equal(t, 2, int(notificationCounter.Load()))
			notificationCounter.Store(0)
		})
	}

	t.Log("Invalid state transition - Accepted -> Completed")
	{
		primaryTask.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.Error(t, err)
	}

	t.Log("Invalid state transition - Accepted -> In-progress, Requester")
	{
		primaryTask.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.Error(t, err)
	}

	t.Log("Valid state transition - Accepted -> In-progress, Owner")
	{
		primaryTask.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor2.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.NoError(t, err)
		primaryTask = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusInProgress, updatedTask.Status)
		})
		t.Run("Check that CareTeam still contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor2, *carePlan.CareTeam[0].Reference, participant1, participant2)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Equal(t, 2, int(notificationCounter.Load()))
			notificationCounter.Store(0)
		})
	}

	t.Log("Complete Task")
	{
		primaryTask.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor2.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.NoError(t, err)
		primaryTask = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusCompleted, updatedTask.Status)
		})
		t.Run("Check that CareTeam now contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1, participant2WithEndDate)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Equal(t, 2, int(notificationCounter.Load()))
			notificationCounter.Store(0)
		})
	}

	t.Log("Creating Task - participant is part of CareTeam and is able to create a primaryTask in an existing CarePlan")
	{
		newTask := fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor2.Create(newTask, &newTask)
		require.NoError(t, err)
		err = carePlanContributor2.Read(*newTask.BasedOn[0].Reference, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, primaryTask.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *newTask.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 3)
			require.Equal(t, "Task", *carePlan.Activity[2].Reference.Type)
			require.Equal(t, "Task/"+*newTask.Id, *carePlan.Activity[2].Reference.Reference)
		})
	}

	// TODO: Will move this into new integ test once Update methods have been implemented
	t.Log("GET patient")
	{
		// Get existing patient
		var fetchedPatient fhir.Patient
		err = carePlanContributor1.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.True(t, coolfhir.IdentifierEquals(&patient.Identifier[0], &fetchedPatient.Identifier[0]))

		// Get non-existing patient
		err = carePlanContributor1.Read("Patient/999", &fetchedPatient)
		require.Error(t, err)

		// Search for existing patient - by ID
		var searchResult fhir.Bundle
		err = carePlanContributor1.Read("Patient", &searchResult, fhirclient.QueryParam("_id", *patient.Id))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id))

		// Search for existing patient - by BSN
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Read("Patient", &searchResult, fhirclient.QueryParam("identifier", "http://fhir.nl/fhir/NamingSystem/bsn|1333333337"))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id))

		// Get existing patient - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Read("Patient/"+*patient2.Id, &fetchedPatient)
		require.Error(t, err)

		// Search for existing patient - by ID - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Read("Patient", &searchResult, fhirclient.QueryParam("_id", *patient2.Id))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 0)

		// Search for existing patient - by BSN - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Read("Patient", &searchResult, fhirclient.QueryParam("identifier", "http://fhir.nl/fhir/NamingSystem/bsn|12345"))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 0)

		searchResult = fhir.Bundle{}
		// Search for patients, one with access one without
		err = carePlanContributor1.Read("Patient", &searchResult, fhirclient.QueryParam("identifier", "http://fhir.nl/fhir/NamingSystem/bsn|1333333337,http://fhir.nl/fhir/NamingSystem/bsn|12345"))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id))
	}

}

func testBundle(t *testing.T, fhirClient *fhirclient.BaseClient, data []byte) {
	var bundle fhir.Bundle
	err := json.Unmarshal(data, &bundle)
	require.NoError(t, err)

	err = fhirClient.Create(bundle, &bundle, fhirclient.AtPath("/"))
	require.NoError(t, err)
	responseData, _ := json.MarshalIndent(bundle, "  ", "")
	println(string(responseData))
}

// These tests only test authorization, field validation is handled by unit tests
func Test_Integration_ResourceAuth(t *testing.T) {
	// TODO: For now, we must rely on unmanaged operations to create these resources, in the future we should be able to create them through an update on non-existing resource
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanContributor1, carePlanContributor2, invalidCarePlanContributor := setupIntegrationTest(t, notificationEndpoint)

	org1Ura := coolfhir.URANamingSystem + "/1"

	patient := fhir.Patient{
		Meta: &fhir.Meta{
			Source: to.Ptr(org1Ura),
		},
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Active: to.Ptr(true),
	}
	t.Log("Create Patient")
	{
		err := carePlanContributor1.Create(patient, &patient)
		require.NoError(t, err)
		require.NotNil(t, patient.Id)

		// Attempting to read the patient fails, as the patient is not yet associated with a CarePlan
		var fetchedPatient fhir.Patient
		err = carePlanContributor1.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.Error(t, err)
	}

	t.Log("Create Task, CarePlan, CareTeam, accept Task - required for auth")
	{
		task := fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
		}

		err := carePlanContributor1.Create(task, &task)
		require.NoError(t, err)

		updatedTask := deep.Copy(task)
		updatedTask.Status = fhir.TaskStatusAccepted
		err = carePlanContributor2.Update("Task/"+*task.Id, updatedTask, &updatedTask)
		require.NoError(t, err)
	}

	t.Log("Patient - GET, UPDATE")
	{
		// Both organizations should be able to read the patient, as both are part of the CareTeam
		var fetchedPatient fhir.Patient
		err := carePlanContributor1.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.True(t, deep.Equal(patient, fetchedPatient))

		err = carePlanContributor2.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.True(t, deep.Equal(patient, fetchedPatient))

		// Only the organization that created the patient should be able to update it
		patientUpdate := deep.Copy(patient)
		patientUpdate.Active = to.Ptr(false)
		err = carePlanContributor2.Update("Patient/"+*patient.Id, patientUpdate, &patientUpdate)
		require.Error(t, err)

		err = carePlanContributor1.Update("Patient/"+*patient.Id, patientUpdate, &patientUpdate)
		require.NoError(t, err)

		err = carePlanContributor1.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.True(t, deep.Equal(patientUpdate, fetchedPatient))

		// A third org without access to the CarePlan/CareTeam can't GET or UPDATE the patient
		err = invalidCarePlanContributor.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.Error(t, err)

		patientUpdate.Active = to.Ptr(true)
		err = invalidCarePlanContributor.Update("Patient/"+*patient.Id, patientUpdate, &patientUpdate)
		require.Error(t, err)
	}

	t.Log("Condition - CREATE, GET, UPDATE")
	{
		condition := fhir.Condition{
			Meta: &fhir.Meta{
				Source: to.Ptr(org1Ura),
			},
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Language: to.Ptr("Nederlands"),
		}

		err := carePlanContributor1.Create(condition, &condition)
		require.NoError(t, err)
		require.NotNil(t, condition.Id)

		// Both organisations have access to the condition
		var fetchedCondition fhir.Condition
		err = carePlanContributor1.Read("Condition/"+*condition.Id, &fetchedCondition)
		require.NoError(t, err)
		require.True(t, deep.Equal(condition, fetchedCondition))

		err = carePlanContributor2.Read("Condition/"+*condition.Id, &fetchedCondition)
		require.NoError(t, err)
		require.True(t, deep.Equal(condition, fetchedCondition))

		// Only the organisation that created the condition can update it
		conditionUpdate := deep.Copy(condition)
		conditionUpdate.Language = to.Ptr("English")
		err = carePlanContributor2.Update("Condition/"+*condition.Id, conditionUpdate, &conditionUpdate)
		require.Error(t, err)

		err = carePlanContributor1.Update("Condition/"+*condition.Id, conditionUpdate, &conditionUpdate)
		require.NoError(t, err)
		err = carePlanContributor1.Read("Condition/"+*condition.Id, &fetchedCondition)
		require.True(t, deep.Equal(conditionUpdate, fetchedCondition))

		// A third org without access to the CarePlan/CareTeam can't GET or UPDATE the condition
		err = invalidCarePlanContributor.Read("Condition/"+*condition.Id, &fetchedCondition)
		require.Error(t, err)

		conditionUpdate.Language = to.Ptr("Nederlands")
		err = invalidCarePlanContributor.Update("Condition/"+*condition.Id, conditionUpdate, &conditionUpdate)
		require.Error(t, err)

	}

	t.Log("Questionnaire - CREATE, GET, UPDATE")
	{
		questionnaire := fhir.Questionnaire{
			Meta: &fhir.Meta{
				Source: to.Ptr(org1Ura),
			},
			Language: to.Ptr("Nederlands"),
		}

		err := carePlanContributor1.Create(questionnaire, &questionnaire)
		require.NoError(t, err)
		require.NotNil(t, questionnaire.Id)

		// Both organisations have access to the questionnaire
		var fetchedQuestionnaire fhir.Questionnaire
		err = carePlanContributor1.Read("Questionnaire/"+*questionnaire.Id, &fetchedQuestionnaire)
		require.NoError(t, err)
		require.True(t, deep.Equal(questionnaire, fetchedQuestionnaire))

		err = carePlanContributor2.Read("Questionnaire/"+*questionnaire.Id, &fetchedQuestionnaire)
		require.NoError(t, err)
		require.True(t, deep.Equal(questionnaire, fetchedQuestionnaire))

		// Only the organisation that created the questionnaire can update it
		questionnaireUpdate := deep.Copy(questionnaire)
		questionnaireUpdate.Language = to.Ptr("English")
		err = carePlanContributor2.Update("Questionnaire/"+*questionnaire.Id, questionnaireUpdate, &questionnaireUpdate)
		require.Error(t, err)

		err = carePlanContributor1.Update("Questionnaire/"+*questionnaire.Id, questionnaireUpdate, &questionnaireUpdate)
		require.NoError(t, err)
		err = carePlanContributor1.Read("Questionnaire/"+*questionnaire.Id, &fetchedQuestionnaire)
		require.True(t, deep.Equal(questionnaireUpdate, fetchedQuestionnaire))

		// Any authorised party can access questionnaire
		err = invalidCarePlanContributor.Read("Questionnaire/"+*questionnaire.Id, &fetchedQuestionnaire)
		require.NoError(t, err)
		require.True(t, deep.Equal(questionnaireUpdate, fetchedQuestionnaire))

		// The third party cannot update the questionnaire
		questionnaireUpdate.Language = to.Ptr("Nederlands")
		err = invalidCarePlanContributor.Update("Questionnaire/"+*questionnaire.Id, questionnaireUpdate, &questionnaireUpdate)
		require.Error(t, err)
	}

	t.Log("QuestionnaireResponse - CREATE, GET, UPDATE")
	{
		questionnaireResponse := fhir.QuestionnaireResponse{
			Meta: &fhir.Meta{
				Source: to.Ptr(org1Ura),
			},
			Status: fhir.QuestionnaireResponseStatusInProgress,
		}
		err := carePlanContributor1.Create(questionnaireResponse, &questionnaireResponse)
		require.NoError(t, err)

		// Both organisations do not have access as the questionnaire response is not related to a task
		var fetchedQuestionnaireResponse fhir.QuestionnaireResponse
		err = carePlanContributor1.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.Error(t, err)

		err = carePlanContributor2.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.Error(t, err)

		// Manually creating a task with the questionnaire response in the output, this is normally handled by the task filler flow
		task := fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Output: []fhir.TaskOutput{
				{
					Type: fhir.CodeableConcept{
						Coding: []fhir.Coding{
							{
								System: to.Ptr("http://terminology.hl7.org/CodeSystem/task-output-type"),
								Code:   to.Ptr("Reference"),
							},
						},
					},
					ValueReference: &fhir.Reference{
						Reference: to.Ptr("QuestionnaireResponse/" + *questionnaireResponse.Id),
					},
				},
			},
		}
		err = carePlanContributor1.Create(task, &task)
		require.NoError(t, err)

		task.Status = fhir.TaskStatusAccepted
		err = carePlanContributor2.Update("Task/"+*task.Id, task, &task)
		require.NoError(t, err)

		err = carePlanContributor1.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.NoError(t, err)
		require.True(t, deep.Equal(questionnaireResponse, fetchedQuestionnaireResponse))

		err = carePlanContributor2.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.NoError(t, err)
		require.True(t, deep.Equal(questionnaireResponse, fetchedQuestionnaireResponse))

		// Only the organisation that created the questionnaire response can update it
		questionnaireResponseUpdate := deep.Copy(questionnaireResponse)
		questionnaireResponseUpdate.Status = fhir.QuestionnaireResponseStatusCompleted
		err = carePlanContributor2.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponseUpdate, &questionnaireResponseUpdate)
		require.Error(t, err)

		err = carePlanContributor1.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponseUpdate, &questionnaireResponseUpdate)
		require.NoError(t, err)
		err = carePlanContributor1.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.True(t, deep.Equal(questionnaireResponseUpdate, fetchedQuestionnaireResponse))

		// A third org without access to the CarePlan/CareTeam can't GET or UPDATE the questionnaire response
		err = invalidCarePlanContributor.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &fetchedQuestionnaireResponse)
		require.Error(t, err)

		questionnaireResponseUpdate.Language = to.Ptr("Nederlands")
		err = invalidCarePlanContributor.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponseUpdate, &questionnaireResponseUpdate)
		require.Error(t, err)
	}

	t.Log("ServiceRequest - CREATE, GET, UPDATE")
	{
		serviceRequest := fhir.ServiceRequest{
			Meta: &fhir.Meta{
				Source: to.Ptr(org1Ura),
			},
			Status: fhir.RequestStatusActive,
		}

		err := carePlanContributor1.Create(serviceRequest, &serviceRequest)
		require.NoError(t, err)

		// Both organisations can't access the service request as it is not related to a task
		var fetchedServiceRequest fhir.ServiceRequest
		err = carePlanContributor1.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.Error(t, err)

		err = carePlanContributor2.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.Error(t, err)

		// Manually creating a task with the questionnaire response in the output, this is normally handled by the task filler flow
		task := fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Focus: &fhir.Reference{
				Reference: to.Ptr("ServiceRequest/" + *serviceRequest.Id),
			},
		}
		err = carePlanContributor1.Create(task, &task)
		require.NoError(t, err)

		// Both organisations have access to the service request
		err = carePlanContributor1.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.NoError(t, err)
		require.True(t, deep.Equal(serviceRequest, fetchedServiceRequest))

		err = carePlanContributor2.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.NoError(t, err)
		require.True(t, deep.Equal(serviceRequest, fetchedServiceRequest))

		// Only the organisation that created the service request can update it
		serviceRequestUpdate := deep.Copy(serviceRequest)
		serviceRequestUpdate.Status = fhir.RequestStatusCompleted
		err = carePlanContributor2.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequestUpdate, &serviceRequestUpdate)
		require.Error(t, err)

		err = carePlanContributor1.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequestUpdate, &serviceRequestUpdate)
		require.NoError(t, err)
		err = carePlanContributor1.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.True(t, deep.Equal(serviceRequestUpdate, fetchedServiceRequest))

		// A third org without access to the CarePlan/CareTeam can't GET or UPDATE the service request
		err = invalidCarePlanContributor.Read("ServiceRequest/"+*serviceRequest.Id, &fetchedServiceRequest)
		require.Error(t, err)

		serviceRequestUpdate.Status = fhir.RequestStatusActive
		err = invalidCarePlanContributor.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequestUpdate, &serviceRequestUpdate)
		require.Error(t, err)
	}

}

func setupIntegrationTest(t *testing.T, notificationEndpoint *url.URL) (*fhirclient.BaseClient, *fhirclient.BaseClient, *fhirclient.BaseClient) {
	fhirBaseURL := test.SetupHAPI(t)
	activeProfile := profile.TestProfile{
		Principal:        auth.TestPrincipal1,
		TestCsdDirectory: profile.TestCsdDirectory{Endpoint: notificationEndpoint.String()},
	}
	config := DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	config.AllowUnmanagedFHIROperations = true
	service, err := New(config, activeProfile, orcaPublicURL)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")

	transport1 := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal1, "")
	transport2 := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, "")
	transport3 := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal3, "")

	carePlanContributor1 := fhirclient.New(carePlanServiceURL, &http.Client{Transport: transport1}, nil)
	carePlanContributor2 := fhirclient.New(carePlanServiceURL, &http.Client{Transport: transport2}, nil)
	carePlanContributor3 := fhirclient.New(carePlanServiceURL, &http.Client{Transport: transport3}, nil)
	return carePlanContributor1, carePlanContributor2, carePlanContributor3
}

func assertCareTeam(t *testing.T, fhirClient fhirclient.Client, careTeamRef string, expectedMembers ...fhir.CareTeamParticipant) {
	t.Helper()

	var careTeam fhir.CareTeam
	err := fhirClient.Read(careTeamRef, &careTeam)
	require.NoError(t, err)
	require.Lenf(t, careTeam.Participant, len(expectedMembers), "expected %d participants, got %d", len(expectedMembers), len(careTeam.Participant))
	for _, participant := range careTeam.Participant {
		require.NoError(t, coolfhir.ValidateLogicalReference(participant.Member, "Organization", coolfhir.URANamingSystem))
	}

outer:
	for _, expectedMember := range expectedMembers {
		for _, participant := range careTeam.Participant {
			if *participant.Member.Identifier.Value == *expectedMember.Member.Identifier.Value {
				// assert Period
				if expectedMember.Period != nil && expectedMember.Period.Start != nil {
					assert.NotNil(t, participant.Period.Start)
				} else {
					assert.Nil(t, participant.Period.Start)
				}
				if expectedMember.Period != nil && expectedMember.Period.End != nil {
					assert.NotNil(t, participant.Period.End)
				} else {
					assert.Nil(t, participant.Period.End)
				}
				continue outer
			}
		}
		t.Errorf("expected participant not found: %s", *expectedMember.Member.Identifier.Value)
	}
}

func setupNotificationEndpoint(t *testing.T) *url.URL {
	notificationEndpoint := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		notificationCounter.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		notificationEndpoint.Close()
	})
	u, _ := url.Parse(notificationEndpoint.URL)
	return u
}
