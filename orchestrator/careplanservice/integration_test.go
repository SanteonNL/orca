package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/messaging"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var patientReference = fhir.Reference{
	Identifier: &fhir.Identifier{
		System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
		Value:  to.Ptr("1333333337"),
	},
}

func Test_Integration(t *testing.T) {
	// Note: this test consists of multiple steps that look like subtests, but they can't be subtests:
	//       in Golang, running a single Subtest causes the other tests not to run.
	//       This causes issues, since each test step (e.g. accepting Task) requires the previous step (test) to succeed (e.g. creating Task).
	t.Log("This test creates a new CarePlan and Task, then runs the Task through requested->accepted->completed lifecycle.")
	var cpc1Notifications []coolfhir.SubscriptionNotification
	cpc1NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {
		cpc1Notifications = append(cpc1Notifications, n)
	})
	var cpc2Notifications []coolfhir.SubscriptionNotification
	cpc2NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {
		cpc2Notifications = append(cpc2Notifications, n)
	})
	fhirBaseURL := test.SetupHAPI(t)
	carePlanContributor1, carePlanContributor2, invalidCarePlanContributor, service := setupIntegrationTest(t, cpc1NotificationEndpoint, cpc2NotificationEndpoint, fhirBaseURL)
	// subTest logs the message and resets the notifications
	subTest := func(t *testing.T, msg string) {
		t.Log(msg)
		cpc1Notifications = nil
		cpc2Notifications = nil
	}

	t.Run("custom search parameters", func(t *testing.T) {
		t.Run("existence", func(t *testing.T) {
			var capabilityStatement fhir.CapabilityStatement
			err := service.fhirClient.Read("metadata", &capabilityStatement)
			require.NoError(t, err)
			for _, rest := range capabilityStatement.Rest {
				for _, resource := range rest.Resource {
					for _, searchParam := range resource.SearchParam {
						if searchParam.Definition != nil && *searchParam.Definition == "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier" {
							// OK
							return
						}
					}
				}
			}
			require.Fail(t, "Search parameter CarePlan-subject-identifier not found")
		})
		t.Run("search parameters already exist", func(t *testing.T) {
			err := service.ensureCustomSearchParametersExists(context.Background())
			require.NoError(t, err)
		})
	})

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
			*patientReference.Identifier,
		},
	}
	err := carePlanContributor1.Create(patient, &patient)
	require.NoError(t, err)

	// Patient not associated with any CarePlans or CareTeams for negative auth testing
	var patient2 fhir.Patient
	err = carePlanContributor1.Create(fhir.Patient{
		Identifier: []fhir.Identifier{
			*patientReference.Identifier,
		},
	}, &patient2)
	require.NoError(t, err)

	var carePlan fhir.CarePlan
	var primaryTask fhir.Task
	subTest(t, "Creating Task - CarePlan does not exist")
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
		require.Empty(t, cpc1Notifications)
		require.Empty(t, cpc2Notifications)
	}

	subTest(t, "Creating Task - No BasedOn, requester is not care organization so creation fails")
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

	subTest(t, "Creating Task - No BasedOn, no Task.For so primaryTask creation fails")
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

	subTest(t, "Creating Task - Task is created through upsert (PUT on non-existing resource)")
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
			For: &patientReference,
		}

		err := carePlanContributor1.Update("Task", primaryTask, &primaryTask, fhirclient.QueryParam("_id", "123"))
		require.NoError(t, err)
		// Resolve created CarePlan through Task.basedOn
		require.NoError(t, carePlanContributor1.Read(*primaryTask.BasedOn[0].Reference, &carePlan))
	}

	t.Run("Conditional Create: Task is not created if it already exists (managed resource)", func(t *testing.T) {
		t.Skip("Implementation broken (INT-529): Task isn't created twice, but it's still added to CarePlan as second activity")
		// Create the Task again with conditional create
		requestHeaders := http.Header{"If-None-Exist": {"_id=" + *primaryTask.Id}}
		err = carePlanContributor1.Create(primaryTask, &primaryTask, fhirclient.RequestHeaders(requestHeaders))
		require.NoError(t, err)

		// Search again, there still should be just 1 Task
		var taskBundle fhir.Bundle
		err = service.fhirClient.SearchWithContext(context.Background(), "Task", url.Values{"intent": {"order"}}, &taskBundle)
		require.NoError(t, err)
		require.Len(t, taskBundle.Entry, 1, "expected 1 Task in the FHIR server")
	})

	subTest(t, "Create Subtask")
	{
		subTask := fhir.Task{
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusReady,
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
			For: &patientReference,
		}
		err := carePlanContributor2.Create(subTask, &subTask)
		require.NoError(t, err)
		// INT-440: CarePlan.activity should not contain subtasks
		require.NoError(t, carePlanContributor1.Read("CarePlan/"+*carePlan.Id, &carePlan))
		require.Len(t, carePlan.Activity, 1)

		// Search for parent task using part-of
		var searchResult fhir.Bundle
		err = carePlanContributor1.Search("Task", url.Values{"part-of": {*subTask.PartOf[0].Reference}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)

		subTest(t, "Completing Subtask")
		{
			subTask.Status = fhir.TaskStatusCompleted
			err := carePlanContributor1.Update("Task/"+*subTask.Id, subTask, &subTask)
			require.NoError(t, err)
		}

		// Here, the Task Filler checks the subtask questionnaire response (if there were any), and then accepts or rejects the Task
		subTest(t, "Accepting Task")
		{
			primaryTask.Status = fhir.TaskStatusAccepted
			err := carePlanContributor2.Update("Task/"+*primaryTask.Id, primaryTask, &primaryTask)
			require.NoError(t, err)
			t.Run("INT-516: check CareTeam contains both parties, and that both are active", func(t *testing.T) {
				careTeam := assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1, participant2)
				assert.NotNil(t, careTeam.Participant[0].Period.Start)
				assert.Nil(t, careTeam.Participant[0].Period.End)
				assert.NotNil(t, careTeam.Participant[1].Period.Start)
				assert.Nil(t, careTeam.Participant[1].Period.End)
			})
		}
	}

	subTest(t, "Creating Task - No BasedOn, new CarePlan and CareTeam are created")
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
			For: &patientReference,
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
			assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 2)
			assertContainsNotification(t, "Task", cpc1Notifications)
			assertContainsNotification(t, "CarePlan", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Search CarePlan")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Search("CarePlan", url.Values{"_id": {*carePlan.Id}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1, "Expected 1 CarePlan")
		require.NoError(t, coolfhir.ResourceInBundle(&searchResult, coolfhir.EntryIsOfType("CarePlan"), new(fhir.CarePlan)))
	}

	subTest(t, "Read CarePlan - Not in participants")
	{
		var fetchedCarePlan fhir.CarePlan
		err := invalidCarePlanContributor.Read("CarePlan/"+*carePlan.Id, &fetchedCarePlan)
		require.Error(t, err)
	}
	subTest(t, "Read CareTeam")
	{
		assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1)
	}
	subTest(t, "Read CareTeam - Does not exist")
	{
		var fetchedCareTeam fhir.CareTeam
		err := carePlanContributor1.Read("CarePlan/999", &fetchedCareTeam)
		require.Error(t, err)
	}
	subTest(t, "Read CareTeam - Not in participants")
	{
		var fetchedCareTeam fhir.CareTeam
		err := invalidCarePlanContributor.Read(*carePlan.CareTeam[0].Reference, &fetchedCareTeam)
		require.Error(t, err)
	}
	subTest(t, "Read Task")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor1.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.NoError(t, err)
		require.NotNil(t, fetchedTask.Id)
		require.Equal(t, fhir.TaskStatusRequested, fetchedTask.Status)
		assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1)
	}
	subTest(t, "Read Task - Non-creating referenced party")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor2.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.NoError(t, err)
		require.NotNil(t, fetchedTask.Id)
		require.Equal(t, fhir.TaskStatusRequested, fetchedTask.Status)
	}
	subTest(t, "Read Task - Not in participants")
	{
		var fetchedTask fhir.Task
		err := invalidCarePlanContributor.Read("Task/"+*primaryTask.Id, &fetchedTask)
		require.Error(t, err)
	}
	subTest(t, "Read Task - Does not exist")
	{
		var fetchedTask fhir.Task
		err := carePlanContributor1.Read("Task/999", &fetchedTask)
		require.Error(t, err)
	}
	previousTask := primaryTask

	subTest(t, "Creating Task - Invalid status Accepted")
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
		require.Empty(t, cpc1Notifications)
		require.Empty(t, cpc2Notifications)
	}

	subTest(t, "Creating Task - Invalid status Draft")
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
			For:       &patientReference,
		}

		err := carePlanContributor1.Create(primaryTask, &primaryTask)
		require.Error(t, err)
		require.Empty(t, cpc1Notifications)
		require.Empty(t, cpc2Notifications)
	}

	subTest(t, "Creating Task - Task.For does not match CarePlan.subject - Fails")
	{
		invalidTask := fhir.Task{
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
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("9876543210"),
				},
			},
		}

		err = carePlanContributor1.Create(invalidTask, &invalidTask)
		require.Error(t, err)
		var operationOutcome fhirclient.OperationOutcomeError
		require.ErrorAs(t, err, &operationOutcome)
		require.Contains(t, *operationOutcome.Issue[0].Diagnostics, "Task.for must reference the same patient as CarePlan.subject")
	}

	subTest(t, "Creating Task - Task.For is nil - Fails")
	{
		invalidTask := fhir.Task{
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

		err = carePlanContributor1.Create(invalidTask, &invalidTask)
		require.Error(t, err)
		var operationOutcome fhirclient.OperationOutcomeError
		require.ErrorAs(t, err, &operationOutcome)
		require.Contains(t, *operationOutcome.Issue[0].Diagnostics, "Task.For must be set with a local reference, or a logical identifier, referencing a patient")
	}

	subTest(t, "Creating Task - Existing CarePlan")
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
			For: &patientReference,
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
			assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 2)
			assertContainsNotification(t, "Task", cpc1Notifications)
			assertContainsNotification(t, "CarePlan", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Accepting Task")
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
			assertCareTeam(t, carePlanContributor2, *carePlan.Id, participant1, participant2)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 1)
			assertContainsNotification(t, "Task", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Invalid state transition - Accepted -> Completed")
	{
		primaryTask.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.Error(t, err)
	}

	subTest(t, "Invalid state transition - Accepted -> In-progress, Requester")
	{
		primaryTask.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor1.Update("Task/"+*primaryTask.Id, primaryTask, &updatedTask)
		require.Error(t, err)
	}

	subTest(t, "Valid state transition - Accepted -> In-progress, Owner")
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
		t.Run("Check that CareTeam now contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor2, *carePlan.Id, participant1, participant2)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 1)
			assertContainsNotification(t, "Task", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Complete Task")
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
			assertCareTeam(t, carePlanContributor1, *carePlan.Id, participant1, participant2WithEndDate)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 1)
			assertContainsNotification(t, "Task", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Creating Task - participant is part of CareTeam and is able to create a primaryTask in an existing CarePlan")
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
			For: &patientReference,
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

	testBundleCreation(t, carePlanContributor1)
}

func testBundleCreation(t *testing.T, carePlanContributor1 *fhirclient.BaseClient) {
	t.Run("creating a bundle with Patient and Task should replace identifier with new local reference", func(t *testing.T) {
		localRef := "urn:uuid:xyz"
		patient := fhir.Patient{
			Identifier: []fhir.Identifier{
				*patientReference.Identifier,
			},
		}
		patientRaw, err := json.Marshal(patient)
		require.NoError(t, err)

		task := fhir.Task{
			For: &fhir.Reference{
				Reference: to.Ptr(localRef),
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("123"),
					Assigner: &fhir.Reference{
						Reference: to.Ptr("Organization/1"),
					},
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
		taskRaw, err := json.Marshal(task)
		require.NoError(t, err)

		bundle := fhir.Bundle{
			Type: fhir.BundleTypeTransaction,
			Entry: []fhir.BundleEntry{
				{
					FullUrl:  to.Ptr(localRef),
					Resource: patientRaw,
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbPOST,
						Url:    "Patient",
					},
				},
				{
					Resource: taskRaw,
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbPOST,
						Url:    "Task",
					},
				},
			},
		}

		var responseBundle fhir.Bundle

		err = carePlanContributor1.Create(bundle, &responseBundle, fhirclient.AtPath("/"))
		require.NoError(t, err)

		require.Len(t, responseBundle.Entry, 2)
		// Verify that task.For has been replaced
		var createdPatient fhir.Patient
		var createdTask fhir.Task
		require.NoError(t, coolfhir.ResourceInBundle(&responseBundle, coolfhir.EntryIsOfType("Patient"), &createdPatient))
		require.NoError(t, coolfhir.ResourceInBundle(&responseBundle, coolfhir.EntryIsOfType("Task"), &createdTask))

		require.Equal(t, "Patient/"+*createdPatient.Id, *createdTask.For.Reference)
		require.NotEqual(t, localRef, *createdTask.For.Reference)

		// Verify that Task.For.Assigner.Identifier is not replaced
		require.Equal(t, "Organization/1", *createdTask.For.Identifier.Assigner.Reference)
	})
}

// Verify that URL
func Test_HandleSearchResource(t *testing.T) {

	// Setup test environment
	fhirBaseURL := test.SetupHAPI(t)
	activeProfile := profile.Test()
	config := DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	config.AllowUnmanagedFHIROperations = true
	service, err := New(config, activeProfile, orcaPublicURL, messaging.NewMemoryBroker())
	require.NoError(t, err)

	ctx := context.Background()
	log.Ctx(ctx).Debug().Msg("Testing handleSearchResource function with real FHIR server")

	patients := []fhir.Patient{
		{
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Smith"),
					Given:  []string{"John"},
				},
			},
			Gender: to.Ptr(fhir.AdministrativeGenderMale),
		},
		{
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Jones"),
					Given:  []string{"Sarah"},
				},
			},
			Gender: to.Ptr(fhir.AdministrativeGenderFemale),
		},
		{
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Brown"),
					Given:  []string{"Michael"},
				},
			},
			Gender: to.Ptr(fhir.AdministrativeGenderMale),
		},
	}

	createdPatients := make([]fhir.Patient, len(patients))
	for i, patient := range patients {
		err = service.fhirClient.Create(patient, &createdPatients[i])
		require.NoError(t, err)
		require.NotNil(t, createdPatients[i].Id)
		log.Ctx(ctx).Debug().Msgf("Created test patient with ID: %s", *createdPatients[i].Id)
	}

	carePlans := []fhir.CarePlan{
		{
			Intent: fhir.CarePlanIntentPlan,
			Subject: fhir.Reference{
				Reference: to.Ptr(fmt.Sprintf("Patient/%s", *createdPatients[0].Id)),
			},
			Title: to.Ptr("Care Plan 1"),
		},
		{
			Intent: fhir.CarePlanIntentPlan,
			Subject: fhir.Reference{
				Reference: to.Ptr(fmt.Sprintf("Patient/%s", *createdPatients[1].Id)),
			},
			Title: to.Ptr("Care Plan 2"),
		},
		{
			Intent: fhir.CarePlanIntentPlan,
			Subject: fhir.Reference{
				Reference: to.Ptr(fmt.Sprintf("Patient/%s", *createdPatients[2].Id)),
			},
			Title: to.Ptr("Care Plan 3"),
		},
	}

	createdCarePlans := make([]fhir.CarePlan, len(carePlans))
	for i, carePlan := range carePlans {
		err = service.fhirClient.Create(carePlan, &createdCarePlans[i])
		require.NoError(t, err)
		require.NotNil(t, createdCarePlans[i].Id)
		log.Ctx(ctx).Debug().Msgf("Created test care plan with ID: %s", *createdCarePlans[i].Id)
	}

	t.Run("search patients with multiple IDs", func(t *testing.T) {
		queryParams := url.Values{"_id": []string{*createdPatients[0].Id, *createdPatients[1].Id}}

		patients, bundle, err := handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, patients, 2)

		patientIDs := []string{*patients[0].Id, *patients[1].Id}
		require.Contains(t, patientIDs, *createdPatients[0].Id)
		require.Contains(t, patientIDs, *createdPatients[1].Id)
	})

	t.Run("search patients with gender parameter", func(t *testing.T) {
		queryParams := url.Values{"gender": []string{fhir.AdministrativeGenderMale.Code()}}

		patients, bundle, err := handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.GreaterOrEqual(t, len(patients), 2) // At least our 2 male test patients

		for _, patient := range patients {
			require.Equal(t, fhir.AdministrativeGenderMale, *patient.Gender)
		}
	})

	t.Run("search patients with ID and gender parameters", func(t *testing.T) {
		queryParams := url.Values{
			"_id":    []string{*createdPatients[0].Id},
			"gender": []string{fhir.AdministrativeGenderMale.Code()},
		}

		patients, bundle, err := handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, patients, 1)
		require.Equal(t, *createdPatients[0].Id, *patients[0].Id)
		require.Equal(t, fhir.AdministrativeGenderMale, *patients[0].Gender)

		// Test with mismatched criteria (ID exists but gender doesn't match)
		queryParams = url.Values{
			"_id":    []string{*createdPatients[0].Id, *createdPatients[2].Id},
			"gender": []string{fhir.AdministrativeGenderFemale.Code()},
		}

		patients, bundle, err = handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Empty(t, patients, "Should return no results when ID and gender don't match")

		// Test with all patients and one gender, should return only patients of that gender
		queryParams = url.Values{
			"_id":    []string{*createdPatients[0].Id, *createdPatients[1].Id, *createdPatients[2].Id},
			"gender": []string{fhir.AdministrativeGenderFemale.Code()},
		}

		patients, bundle, err = handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, patients, 1)
		require.Equal(t, *createdPatients[1].Id, *patients[0].Id)
		require.Equal(t, fhir.AdministrativeGenderFemale, *patients[0].Gender)

		// Test with all patients and all genders, should return all patients
		queryParams = url.Values{
			"_id":    []string{*createdPatients[0].Id, *createdPatients[1].Id, *createdPatients[2].Id},
			"gender": []string{fhir.AdministrativeGenderMale.Code(), fhir.AdministrativeGenderFemale.Code()},
		}

		patients, bundle, err = handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, patients, 3)
	})

	t.Run("search careplans with multiple parameters", func(t *testing.T) {
		queryParams := url.Values{"subject": []string{*createdPatients[0].Id, *createdPatients[1].Id}}

		carePlans, bundle, err := handleSearchResource[fhir.CarePlan](ctx, service, "CarePlan", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, carePlans, 2)

		for _, cp := range carePlans {
			subjectRef := *cp.Subject.Reference
			require.True(t,
				subjectRef == fmt.Sprintf("Patient/%s", *createdPatients[0].Id) ||
					subjectRef == fmt.Sprintf("Patient/%s", *createdPatients[1].Id),
				"Unexpected subject reference: %s", subjectRef)
		}
	})

	t.Run("search with custom headers", func(t *testing.T) {
		queryParams := url.Values{"_id": []string{*createdPatients[0].Id}}

		customHeaders := &fhirclient.Headers{}

		patients, bundle, err := handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, customHeaders)

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Len(t, patients, 1)
		require.Equal(t, *createdPatients[0].Id, *patients[0].Id)
	})

	t.Run("search with non-existent ID", func(t *testing.T) {
		queryParams := url.Values{"_id": []string{"non-existent-id"}}

		patients, bundle, err := handleSearchResource[fhir.Patient](ctx, service, "Patient", queryParams, &fhirclient.Headers{})

		require.NoError(t, err)
		require.NotNil(t, bundle)
		require.Empty(t, patients)
	})
}

func setupIntegrationTest(t *testing.T, cpc1NotificationEndpoint string, cpc2NotificationEndpoint string, fhirBaseURL *url.URL) (*fhirclient.BaseClient, *fhirclient.BaseClient, *fhirclient.BaseClient, *Service) {
	activeProfile := profile.TestProfile{
		Principal: auth.TestPrincipal1,
		CSD: profile.TestCsdDirectory{
			Endpoints: map[string]map[string]string{
				auth.TestPrincipal1.ID(): {
					"fhirNotificationURL": cpc1NotificationEndpoint,
				},
				auth.TestPrincipal2.ID(): {
					"fhirNotificationURL": cpc2NotificationEndpoint,
				},
			},
		},
	}
	config := DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	config.AllowUnmanagedFHIROperations = true
	service, err := New(config, activeProfile, orcaPublicURL, messaging.NewMemoryBroker())
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
	return carePlanContributor1, carePlanContributor2, carePlanContributor3, service
}

func assertCareTeam(t *testing.T, fhirClient fhirclient.Client, carePlanId string, expectedMembers ...fhir.CareTeamParticipant) *fhir.CareTeam {
	t.Helper()

	var carePlan fhir.CarePlan

	err := fhirClient.Read(fmt.Sprintf("CarePlan/%s", carePlanId), &carePlan)
	if err != nil {
		t.Fatal(err)
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	if err != nil {
		t.Fatal(err)
	}

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

	return careTeam
}

func setupNotificationEndpoint(t *testing.T, handler func(n coolfhir.SubscriptionNotification)) string {
	notificationEndpoint := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		notificationData, err := io.ReadAll(request.Body)
		require.NoError(t, err)
		t.Logf("Received notification: %s", notificationData)
		var notification coolfhir.SubscriptionNotification
		if err := json.Unmarshal(notificationData, &notification); err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		handler(notification)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		notificationEndpoint.Close()
	})
	return notificationEndpoint.URL
}

func assertContainsNotification(t *testing.T, resourceType string, notifications []coolfhir.SubscriptionNotification) {
	t.Helper()
	for _, notification := range notifications {
		focus, err := notification.GetFocus()
		require.NoError(t, err)
		if focus.Type != nil && *focus.Type == resourceType {
			return
		}
	}
	assert.Fail(t, "notification not found", "expected notification for resource type %s", resourceType)
}
