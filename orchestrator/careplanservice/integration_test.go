package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"io"
	"math/rand"
	"strconv"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
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
	carePlanContributor1, carePlanContributor2, invalidCarePlanContributor, service := setupIntegrationTest(t, cpc1NotificationEndpoint, cpc2NotificationEndpoint)
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

	t.Run("DELETE is supported when unmanaged operations are allowed", func(t *testing.T) {
		serviceRequest := fhir.ServiceRequest{
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr(strconv.Itoa(rand.Int())),
				},
			},
		}
		t.Run("as single request", func(t *testing.T) {
			t.Run("with ID", func(t *testing.T) {
				var actual fhir.ServiceRequest
				err := carePlanContributor1.Create(serviceRequest, &actual)
				require.NoError(t, err)

				// Find and delete audit events first due to referential integrity
				var auditEvents fhir.Bundle
				err = carePlanContributor1.Search("AuditEvent", url.Values{
					"entity": {"ServiceRequest/" + *actual.Id},
				}, &auditEvents)
				require.NoError(t, err)

				// Delete each audit event referencing this resource
				for _, entry := range auditEvents.Entry {
					var auditEvent fhir.AuditEvent
					err = json.Unmarshal(entry.Resource, &auditEvent)
					require.NoError(t, err)
					err = carePlanContributor1.Delete("AuditEvent/"+*auditEvent.Id, nil)
					require.NoError(t, err)
				}

				// Now delete the resource
				err = carePlanContributor1.Delete("ServiceRequest/"+*actual.Id, nil)
				require.NoError(t, err)
			})
			t.Run("with ID as search parameter", func(t *testing.T) {
				var actual fhir.ServiceRequest
				err := carePlanContributor1.Create(serviceRequest, &actual)
				require.NoError(t, err)

				// Find and delete audit events first
				var auditEvents fhir.Bundle
				err = carePlanContributor1.Search("AuditEvent", url.Values{
					"entity": {"ServiceRequest/" + *actual.Id},
				}, &auditEvents)
				require.NoError(t, err)

				for _, entry := range auditEvents.Entry {
					var auditEvent fhir.AuditEvent
					err = json.Unmarshal(entry.Resource, &auditEvent)
					require.NoError(t, err)
					err = carePlanContributor1.Delete("AuditEvent/"+*auditEvent.Id, nil)
					require.NoError(t, err)
				}

				// Then delete the resource
				err = carePlanContributor1.Delete("ServiceRequest", fhirclient.QueryParam("_id", *actual.Id))
				require.NoError(t, err)
			})
		})
		t.Run("as transaction entry", func(t *testing.T) {
			var actual fhir.ServiceRequest
			err := carePlanContributor1.Create(serviceRequest, &actual)
			require.NoError(t, err)

			// Find audit events
			var auditEvents fhir.Bundle
			err = carePlanContributor1.Search("AuditEvent", url.Values{
				"entity": {"ServiceRequest/" + *actual.Id},
			}, &auditEvents)
			require.NoError(t, err)

			// Create transaction to delete audit events first, then the resource
			transaction := fhir.Bundle{
				Type:  fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{},
			}

			// Add audit event deletions to transaction
			for _, entry := range auditEvents.Entry {
				var auditEvent fhir.AuditEvent
				err = json.Unmarshal(entry.Resource, &auditEvent)
				require.NoError(t, err)

				transaction.Entry = append(transaction.Entry, fhir.BundleEntry{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbDELETE,
						Url:    "AuditEvent/" + *auditEvent.Id,
					},
				})
			}

			// Add resource deletion to transaction
			transaction.Entry = append(transaction.Entry, fhir.BundleEntry{
				Request: &fhir.BundleEntryRequest{
					Method: fhir.HTTPVerbDELETE,
					Url:    "ServiceRequest/" + *actual.Id,
				},
			})

			err = carePlanContributor1.Create(transaction, &transaction, fhirclient.AtPath("/"))
			require.NoError(t, err)
		})
	})

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
				var careTeam fhir.CareTeam
				err := carePlanContributor1.Read(*carePlan.CareTeam[0].Reference, &careTeam)
				require.NoError(t, err)
				assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1, participant2)
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
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 3)
			assertContainsNotification(t, "Task", cpc1Notifications)
			assertContainsNotification(t, "CareTeam", cpc1Notifications)
			assertContainsNotification(t, "CarePlan", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Search CarePlan")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Search("CarePlan", url.Values{"_id": {*carePlan.Id}, "_include": {"CarePlan:care-team"}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 2, "Expected 1 CarePlan and 1 CareTeam")
		require.NoError(t, coolfhir.ResourceInBundle(&searchResult, coolfhir.EntryIsOfType("CarePlan"), new(fhir.CarePlan)))
		require.NoError(t, coolfhir.ResourceInBundle(&searchResult, coolfhir.EntryIsOfType("CareTeam"), new(fhir.CareTeam)))
	}

	subTest(t, "Read CarePlan - Not in participants")
	{
		var fetchedCarePlan fhir.CarePlan
		err := invalidCarePlanContributor.Read("CarePlan/"+*carePlan.Id, &fetchedCarePlan)
		require.Error(t, err)
	}
	subTest(t, "Read CareTeam")
	{
		var fetchedCareTeam fhir.CareTeam
		err := carePlanContributor1.Read(*carePlan.CareTeam[0].Reference, &fetchedCareTeam)
		require.NoError(t, err)
		assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
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
		assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
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
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1)
		})
		t.Run("Check that 2 parties have been notified", func(t *testing.T) {
			require.Len(t, cpc1Notifications, 2)
			assertContainsNotification(t, "Task", cpc1Notifications)
			assertContainsNotification(t, "CarePlan", cpc1Notifications)
			require.Len(t, cpc2Notifications, 1)
			assertContainsNotification(t, "Task", cpc2Notifications)
		})
	}

	subTest(t, "Care Team Search")
	{
		var searchResult fhir.Bundle
		err := carePlanContributor1.Search("CareTeam", url.Values{}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 2, "Expected 1 team")
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
			assertCareTeam(t, carePlanContributor2, *carePlan.CareTeam[0].Reference, participant1, participant2)
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
			assertCareTeam(t, carePlanContributor2, *carePlan.CareTeam[0].Reference, participant1, participant2)
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
			assertCareTeam(t, carePlanContributor1, *carePlan.CareTeam[0].Reference, participant1, participant2WithEndDate)
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

	// TODO: Will move this into new integ test once Update methods have been implemented
	subTest(t, "GET patient")
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
		err = carePlanContributor1.Search("Patient", url.Values{"_id": {*patient.Id}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id))

		// Search for existing patient - by BSN
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Search("Patient", url.Values{"identifier": {"http://fhir.nl/fhir/NamingSystem/bsn|1333333337"}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.True(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id))

		// Get existing patient - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Read("Patient/"+*patient2.Id, &fetchedPatient)
		require.Error(t, err)

		// Search for existing patient - by ID - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Search("Patient", url.Values{"_id": {*patient2.Id}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 0)

		// Search for existing patient - by BSN - no access
		searchResult = fhir.Bundle{}
		err = carePlanContributor1.Search("Patient", url.Values{"identifier": {"http://fhir.nl/fhir/NamingSystem/bsn|12345"}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 0)

		searchResult = fhir.Bundle{}
		// Search for patients, one with access one without
		err = carePlanContributor1.Search("Patient", url.Values{"identifier": {"http://fhir.nl/fhir/NamingSystem/bsn|1333333337,http://fhir.nl/fhir/NamingSystem/bsn|12345"}}, &searchResult)
		require.NoError(t, err)
		require.Len(t, searchResult.Entry, 1)
		require.Truef(t, strings.HasSuffix(*searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id), "Expected %s to end with %s", *searchResult.Entry[0].FullUrl, "Patient/"+*patient.Id)
	}

	testBundleCreation(t, carePlanContributor1)
}

func Test_CRUD_AuditEvents(t *testing.T) {
	// Setup test clients and service
	cpc1NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	cpc2NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	carePlanContributor1, carePlanContributor2, _, _ := setupIntegrationTest(t, cpc1NotificationEndpoint, cpc2NotificationEndpoint)

	// Track all expected audit events
	var expectedAuditEvents []ExpectedAuditEvent

	// Helper to add expected audit events
	addExpectedAudit := func(resourceRef string, action fhir.AuditEventAction) {
		expectedAuditEvents = append(expectedAuditEvents, ExpectedAuditEvent{
			ResourceRef: resourceRef,
			Action:      action,
		})
	}

	// Create Patient
	patient := fhir.Patient{
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Name: []fhir.HumanName{
			{
				Given:  []string{"Test"},
				Family: to.Ptr("Patient"),
			},
		},
	}
	err := carePlanContributor1.Create(patient, &patient)
	require.NoError(t, err)
	addExpectedAudit("Patient/"+*patient.Id, fhir.AuditEventActionC)

	// Create Questionnaire
	questionnaire := fhir.Questionnaire{
		Status: fhir.PublicationStatusDraft,
		Title:  to.Ptr("Test Questionnaire"),
		Item: []fhir.QuestionnaireItem{
			{
				LinkId: "1",
				Text:   to.Ptr("Question 1"),
				Type:   fhir.QuestionnaireItemTypeString,
			},
		},
	}
	err = carePlanContributor1.Create(questionnaire, &questionnaire)
	require.NoError(t, err)
	addExpectedAudit("Questionnaire/"+*questionnaire.Id, fhir.AuditEventActionC)

	// Create QuestionnaireResponse
	questionnaireResponse := fhir.QuestionnaireResponse{
		Status:        fhir.QuestionnaireResponseStatusInProgress,
		Questionnaire: to.Ptr("Questionnaire/" + *questionnaire.Id),
		Subject: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	err = carePlanContributor1.Create(questionnaireResponse, &questionnaireResponse)
	require.NoError(t, err)
	addExpectedAudit("QuestionnaireResponse/"+*questionnaireResponse.Id, fhir.AuditEventActionC)

	// Create ServiceRequest
	serviceRequest := fhir.ServiceRequest{
		Status: fhir.RequestStatusActive,
		Intent: fhir.RequestIntentOrder,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Code: &fhir.CodeableConcept{
			Text: to.Ptr("Test Service"),
		},
	}
	err = carePlanContributor1.Create(serviceRequest, &serviceRequest)
	require.NoError(t, err)
	addExpectedAudit("ServiceRequest/"+*serviceRequest.Id, fhir.AuditEventActionC)

	// Update Patient
	patient.Name[0].Given = []string{"Updated"}
	err = carePlanContributor1.Update("Patient/"+*patient.Id, patient, &patient)
	require.NoError(t, err)
	addExpectedAudit("Patient/"+*patient.Id, fhir.AuditEventActionU)

	// Update Questionnaire
	questionnaire.Title = to.Ptr("Updated Questionnaire")
	err = carePlanContributor1.Update("Questionnaire/"+*questionnaire.Id, questionnaire, &questionnaire)
	require.NoError(t, err)
	addExpectedAudit("Questionnaire/"+*questionnaire.Id, fhir.AuditEventActionU)

	// Update QuestionnaireResponse
	questionnaireResponse.Status = fhir.QuestionnaireResponseStatusCompleted
	err = carePlanContributor1.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponse, &questionnaireResponse)
	require.NoError(t, err)
	addExpectedAudit("QuestionnaireResponse/"+*questionnaireResponse.Id, fhir.AuditEventActionU)

	// Update ServiceRequest
	serviceRequest.Status = fhir.RequestStatusCompleted
	err = carePlanContributor1.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequest, &serviceRequest)
	require.NoError(t, err)
	addExpectedAudit("ServiceRequest/"+*serviceRequest.Id, fhir.AuditEventActionU)

	// Negative tests - different user trying to update resources
	t.Run("Update Patient with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("Patient/"+*patient.Id, patient, &patient)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this Patient")
	})

	t.Run("Update Questionnaire with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("Questionnaire/"+*questionnaire.Id, questionnaire, &questionnaire)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this Questionnaire")
	})

	t.Run("Update QuestionnaireResponse with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponse, &questionnaireResponse)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this QuestionnaireResponse")
	})

	t.Run("Update ServiceRequest with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequest, &serviceRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this ServiceRequest")
	})

	// Update non-existing resources (creates new ones)
	t.Run("Update non-existing Patient - creates new resource", func(t *testing.T) {
		nonExistingPatient := fhir.Patient{
			Id: to.Ptr("non-existing-patient"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333338"),
				},
			},
			Name: []fhir.HumanName{
				{
					Given:  []string{"New"},
					Family: to.Ptr("Patient"),
				},
			},
		}
		err = carePlanContributor1.Update("Patient/"+*nonExistingPatient.Id, nonExistingPatient, &nonExistingPatient)
		require.NoError(t, err)
		addExpectedAudit("Patient/"+*nonExistingPatient.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing Questionnaire - creates new resource", func(t *testing.T) {
		nonExistingQuestionnaire := fhir.Questionnaire{
			Id:     to.Ptr("non-existing-questionnaire"),
			Status: fhir.PublicationStatusDraft,
			Title:  to.Ptr("New Test Questionnaire"),
			Item: []fhir.QuestionnaireItem{
				{
					LinkId: "1",
					Text:   to.Ptr("New Question 1"),
					Type:   fhir.QuestionnaireItemTypeString,
				},
			},
		}
		err = carePlanContributor1.Update("Questionnaire/"+*nonExistingQuestionnaire.Id, nonExistingQuestionnaire, &nonExistingQuestionnaire)
		require.NoError(t, err)
		addExpectedAudit("Questionnaire/"+*nonExistingQuestionnaire.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing QuestionnaireResponse - creates new resource", func(t *testing.T) {
		nonExistingQuestionnaireResponse := fhir.QuestionnaireResponse{
			Id:            to.Ptr("non-existing-questionnaire-response"),
			Status:        fhir.QuestionnaireResponseStatusInProgress,
			Questionnaire: to.Ptr("Questionnaire/" + *questionnaire.Id),
			Subject: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
		}
		err = carePlanContributor1.Update("QuestionnaireResponse/"+*nonExistingQuestionnaireResponse.Id, nonExistingQuestionnaireResponse, &nonExistingQuestionnaireResponse)
		require.NoError(t, err)
		addExpectedAudit("QuestionnaireResponse/"+*nonExistingQuestionnaireResponse.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing ServiceRequest - creates new resource", func(t *testing.T) {
		nonExistingServiceRequest := fhir.ServiceRequest{
			Id:     to.Ptr("non-existing-service-request"),
			Intent: fhir.RequestIntentOrder,
			Status: fhir.RequestStatusDraft,
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Code: &fhir.CodeableConcept{
				Text: to.Ptr("New Service Request"),
			},
		}
		err = carePlanContributor1.Update("ServiceRequest/"+*nonExistingServiceRequest.Id, nonExistingServiceRequest, &nonExistingServiceRequest)
		require.NoError(t, err)
		addExpectedAudit("ServiceRequest/"+*nonExistingServiceRequest.Id, fhir.AuditEventActionC)
	})

	// Verify all audit events at the end
	err = verifyAuditEvents(t, carePlanContributor1, expectedAuditEvents)
	require.NoError(t, err)
}

// Define a new type to hold expected audit events without timestamp requirements
type ExpectedAuditEvent struct {
	ResourceRef string
	Action      fhir.AuditEventAction
	QueryParams map[string][]string
}

// Refactored verifyAuditEvents to handle a list of expected audit events without timestamp requirements
func verifyAuditEvents(t *testing.T, fhirClient fhirclient.Client, expectedEvents []ExpectedAuditEvent) error {
	t.Helper()

	// Create a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Log the search attempt for debugging
	t.Logf("Searching for AuditEvents")

	var bundle fhir.Bundle
	err := fhirClient.SearchWithContext(ctx, "AuditEvent", url.Values{}, &bundle)

	if err != nil {
		return fmt.Errorf("failed to search AuditEvents: %w", err)
	}

	// Log success for debugging
	t.Logf("Successfully retrieved %d AuditEvents", len(bundle.Entry))

	// Track which expected events have been found
	foundEvents := make(map[string]bool)

	// Process each audit event in the bundle
	for _, entry := range bundle.Entry {
		var auditEvent fhir.AuditEvent
		if err := json.Unmarshal(entry.Resource, &auditEvent); err != nil {
			return fmt.Errorf("failed to unmarshal AuditEvent: %w", err)
		}

		// Skip if no entities or action
		if len(auditEvent.Entity) == 0 || auditEvent.Action == nil {
			continue
		}

		// Check each entity in the audit event
		for _, entity := range auditEvent.Entity {
			if entity.What == nil || entity.What.Reference == nil {
				continue
			}

			resourceRef := *entity.What.Reference
			actionKey := fmt.Sprintf("%s:%s", resourceRef, *auditEvent.Action)

			// Check if this matches any expected event
			for _, expectedEvent := range expectedEvents {
				expectedKey := fmt.Sprintf("%s:%s", expectedEvent.ResourceRef, expectedEvent.Action)

				if actionKey == expectedKey {
					// Check query parameters if needed
					if expectedEvent.QueryParams != nil && *auditEvent.Action == fhir.AuditEventActionE {
						paramsMatch := verifyQueryParams(auditEvent, expectedEvent.QueryParams)
						if !paramsMatch {
							continue
						}
					}

					// Mark this expected event as found
					foundEvents[expectedKey] = true
					break
				}
			}
		}
	}

	// Check if all expected events were found
	for _, event := range expectedEvents {
		key := fmt.Sprintf("%s:%s", event.ResourceRef, event.Action)
		if !foundEvents[key] {
			return fmt.Errorf("expected to find audit event with action %s for resource %s",
				event.Action, event.ResourceRef)
		}
	}

	return nil
}

// Helper function to verify query parameters in an audit event
func verifyQueryParams(auditEvent fhir.AuditEvent, queryParams map[string][]string) bool {
	// Find query entity
	for _, e := range auditEvent.Entity {
		if e.Type != nil && e.Type.Code != nil && *e.Type.Code == "2" { // "2" is the code for query parameters
			// Verify all expected params exist in details
			for param, values := range queryParams {
				paramFound := false
				for _, detail := range e.Detail {
					if detail.Type == param && *detail.ValueString == strings.Join(values, ",") {
						paramFound = true
						break
					}
				}
				if !paramFound {
					return false
				}
			}
			return true
		}
	}
	return false
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

func setupIntegrationTest(t *testing.T, cpc1NotificationEndpoint string, cpc2NotificationEndpoint string) (*fhirclient.BaseClient, *fhirclient.BaseClient, *fhirclient.BaseClient, *Service) {
	fhirBaseURL := test.SetupHAPI(t)
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
	return carePlanContributor1, carePlanContributor2, carePlanContributor3, service
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
