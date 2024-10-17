package careplanservice

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	participant1 := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
		Period:     &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2 := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period:     &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2WithEndDate := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period: &fhir.Period{
			Start: to.Ptr("2021-01-01T00:00:00Z"),
			End:   to.Ptr("2021-01-02T00:00:00Z"),
		},
	}

	var carePlan fhir.CarePlan
	var task fhir.Task
	t.Log("Creating Task - CarePlan does not exist")
	{
		task = fhir.Task{
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
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor1.Create(task, &task)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - No BasedOn, requester is not care organization so creation fails")
	{
		task = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor2.Create(task, &task)
		require.Error(t, err)
	}

	t.Log("Creating Task - No BasedOn, new CarePlan and CareTeam are created")
	{
		task = fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			},
			Focus: &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			},
		}

		err := carePlanContributor1.Create(task, &task)
		require.NoError(t, err)
		err = carePlanContributor1.Read(*task.BasedOn[0].Reference, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, task.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 1)
			require.Equal(t, "Task", *carePlan.Activity[0].Reference.Type)
			require.Equal(t, "Task/"+*task.Id, *carePlan.Activity[0].Reference.Reference)
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
		err := carePlanContributor1.Read("CarePlan", &searchResult, fhirclient.QueryParam("_id", *carePlan.Id))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 2, "Expected 1 CarePlan and 1 CareTeam")
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "CarePlan/"+*carePlan.Id))
		require.True(t, strings.HasSuffix(*searchResult.Entry[1].FullUrl, *carePlan.CareTeam[0].Reference))
	}

	t.Log("Search Subtasks")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Read("Task", &searchResult, fhirclient.QueryParam("part-of", "Task/"+*task.Id))
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1, "Expected 1 subtask")
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
		err := carePlanContributor1.Read("Task/"+*task.Id, &fetchedTask)
		require.NoError(t, err)
		require.NotNil(t, fetchedTask.Id)
		require.Equal(t, fhir.TaskStatusRequested, fetchedTask.Status)
		assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
	}
	t.Log("Read Task - Not in participants")
	{
		var fetchedTask fhir.Task
		err := invalidCarePlanContributor.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Read Task - Does not exist")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor1.Read("Task/999", &fetchedTask)
		require.Error(t, err)
	}
	previousTask := task

	t.Log("Creating Task - Invalid status Accepted")
	{
		task = fhir.Task{
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

		err := carePlanContributor1.Create(task, &task)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - Invalid status Draft")
	{
		task = fhir.Task{
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

		err := carePlanContributor1.Create(task, &task)
		require.Error(t, err)
		require.Equal(t, 0, int(notificationCounter.Load()))
	}

	t.Log("Creating Task - Existing CarePlan")
	{
		task = fhir.Task{
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
		}

		err := carePlanContributor1.Create(task, &task)
		require.NoError(t, err)
		err = carePlanContributor1.Read("CarePlan/"+*carePlan.Id, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, task.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 2)
			for _, activity := range carePlan.Activity {
				require.Equal(t, "Task", *activity.Reference.Type)
				require.Equal(t, true, "Task/"+*task.Id == *activity.Reference.Reference || "Task/"+*previousTask.Id == *activity.Reference.Reference)
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
		require.Len(t, searchResult.Entry, 1, "Expected 1 team")
	}

	t.Log("Accepting Task")
	{
		task.Status = fhir.TaskStatusAccepted
		var updatedTask fhir.Task
		err := carePlanContributor2.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

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
		task.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*task.Id, task, &updatedTask)
		require.Error(t, err)
	}

	t.Log("Invalid state transition - Accepted -> In-progress, Requester")
	{
		task.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*task.Id, task, &updatedTask)
		require.Error(t, err)
	}

	t.Log("Valid state transition - Accepted -> In-progress, Owner")
	{
		task.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor2.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

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
		task.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor2.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

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
		require.NoError(t, coolfhir.ValidateLogicalReference(participant.OnBehalfOf, "Organization", coolfhir.URANamingSystem))
	}

outer:
	for _, expectedMember := range expectedMembers {
		for _, participant := range careTeam.Participant {
			if *participant.OnBehalfOf.Identifier.Value == *expectedMember.OnBehalfOf.Identifier.Value {
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
		t.Errorf("expected participant not found: %s", *expectedMember.OnBehalfOf.Identifier.Value)
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
