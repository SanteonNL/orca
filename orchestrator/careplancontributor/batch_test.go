package careplancontributor

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestService_handleBatch(t *testing.T) {
	httpRequest, _ := http.NewRequest(http.MethodGet, "/", nil)
	httpRequest.Header.Add("X-Scp-Context", "valid")
	tenant := tenants.Test().Sole()
	t.Run("upstream server returns error", func(t *testing.T) {
		fhirClient := test.StubFHIRClient{}
		fhirClient.Error = fhirclient.OperationOutcomeError{
			OperationOutcome: fhir.OperationOutcome{
				Issue: []fhir.OperationOutcomeIssue{
					{
						Severity: fhir.IssueSeverityError,
					},
				},
			},
			HttpStatusCode: 500,
		}
		s := &Service{}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/123",
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, &fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 1)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "500 Internal Server Error", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Response.Outcome)
		var outcome fhir.OperationOutcome
		require.NoError(t, json.Unmarshal(actual.Entry[0].Response.Outcome, &outcome))
		require.Len(t, outcome.Issue, 1)
		require.Equal(t, fhir.IssueSeverityError, outcome.Issue[0].Severity)
	})
	t.Run("non-GET request", func(t *testing.T) {
		s := &Service{
			ehrFHIRClientByTenant: map[string]fhirclient.Client{
				tenant.ID: &test.StubFHIRClient{},
			},
		}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbPOST,
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, nil)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 1)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "400 Bad Request", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Response.Outcome)
		var outcome fhir.OperationOutcome
		require.NoError(t, json.Unmarshal(actual.Entry[0].Response.Outcome, &outcome))
		require.Len(t, outcome.Issue, 1)
		require.Equal(t, fhir.IssueSeverityError, outcome.Issue[0].Severity)
		require.Equal(t, "Only GET requests are supported in batch processing", *outcome.Issue[0].Details.Text)
	})
	t.Run("parallel - simple", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
				fhir.Task{
					Id: to.Ptr("456"),
				},
			},
		}
		s := &Service{
			config: Config{
				ParallelBatch: true,
			},
		}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/123",
					},
				},
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/456",
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 2)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "200 OK", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Resource)

		var entry1 fhir.Task
		require.NoError(t, json.Unmarshal(actual.Entry[0].Resource, &entry1))
		require.Equal(t, "123", *entry1.Id)
		var entry2 fhir.Task
		require.NoError(t, json.Unmarshal(actual.Entry[1].Resource, &entry2))
		require.Equal(t, "456", *entry2.Id)
	})
	t.Run("parallel with mixed resources and ordering verification", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				// Tasks
				fhir.Task{
					Id:     to.Ptr("task-001"),
					Status: fhir.TaskStatusDraft,
				},
				fhir.Task{
					Id:     to.Ptr("task-002"),
					Status: fhir.TaskStatusInProgress,
				},
				fhir.Task{
					Id:     to.Ptr("task-003"),
					Status: fhir.TaskStatusCompleted,
				},
				// Patients
				fhir.Patient{
					Id:     to.Ptr("patient-001"),
					Active: to.Ptr(true),
				},
				fhir.Patient{
					Id:     to.Ptr("patient-002"),
					Active: to.Ptr(true),
				},
				fhir.Patient{
					Id:     to.Ptr("patient-003"),
					Active: to.Ptr(false),
				},
				// CarePlans
				fhir.CarePlan{
					Id:     to.Ptr("careplan-001"),
					Intent: fhir.CarePlanIntentPlan,
				},
				fhir.CarePlan{
					Id:     to.Ptr("careplan-002"),
					Intent: fhir.CarePlanIntentPlan,
				},
				fhir.CarePlan{
					Id:     to.Ptr("careplan-003"),
					Intent: fhir.CarePlanIntentPlan,
				},
				// Conditions
				fhir.Condition{
					Id: to.Ptr("condition-001"),
					ClinicalStatus: &fhir.CodeableConcept{
						Text: to.Ptr("active"),
					},
				},
				fhir.Condition{
					Id: to.Ptr("condition-002"),
					ClinicalStatus: &fhir.CodeableConcept{
						Text: to.Ptr("resolved"),
					},
				},
				fhir.Condition{
					Id: to.Ptr("condition-003"),
					ClinicalStatus: &fhir.CodeableConcept{
						Text: to.Ptr("inactive"),
					},
				},
				// Additional mixed resources
				fhir.Task{
					Id:     to.Ptr("task-004"),
					Status: fhir.TaskStatusReady,
				},
				fhir.Patient{
					Id:     to.Ptr("patient-004"),
					Active: to.Ptr(true),
				},
				fhir.Condition{
					Id: to.Ptr("condition-004"),
					ClinicalStatus: &fhir.CodeableConcept{
						Text: to.Ptr("recurrence"),
					},
				},
			},
		}

		s := &Service{
			config: Config{
				ParallelBatch: true,
			},
		}

		// Define expected order of requests and their corresponding resource IDs
		expectedResourceIds := []string{
			"task-001", "patient-001", "careplan-001", "condition-001", "task-002",
			"patient-002", "careplan-002", "condition-002", "task-003", "patient-003",
			"careplan-003", "condition-003", "task-004", "patient-004", "condition-004",
		}

		// Build request bundle with mixed resource types in specific order
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/task-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/patient-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "CarePlan/careplan-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/condition-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/task-002"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/patient-002"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "CarePlan/careplan-002"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/condition-002"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/task-003"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/patient-003"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "CarePlan/careplan-003"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/condition-003"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/task-004"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/patient-004"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/condition-004"}},
			},
		}

		actual, err := s.doHandleBatch(httpRequest, requestBundle, fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 15, "Should return exactly 15 entries")

		// Verify all entries have successful responses
		for i, entry := range actual.Entry {
			require.NotNil(t, entry.Response, "Entry %d should have a response", i)
			require.Equal(t, "200 OK", entry.Response.Status, "Entry %d should have 200 OK status", i)
			require.NotNil(t, entry.Resource, "Entry %d should have a resource", i)
		}

		// Verify correct ordering by checking resource IDs match expected order
		for i, expectedId := range expectedResourceIds {
			var resourceWithId struct {
				Id string `json:"id"`
			}
			require.NoError(t, json.Unmarshal(actual.Entry[i].Resource, &resourceWithId),
				"Should be able to unmarshal resource at index %d", i)
			require.Equal(t, expectedId, resourceWithId.Id,
				"Resource at index %d should have ID %s", i, expectedId)
		}
	})
	t.Run("parallel with mixed success and failure scenarios", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				// Existing resources
				fhir.Task{
					Id:     to.Ptr("existing-task-001"),
					Status: fhir.TaskStatusDraft,
				},
				fhir.Patient{
					Id:     to.Ptr("existing-patient-001"),
					Active: to.Ptr(true),
				},
				fhir.CarePlan{
					Id:     to.Ptr("existing-careplan-001"),
					Intent: fhir.CarePlanIntentPlan,
				},
				fhir.Condition{
					Id: to.Ptr("existing-condition-001"),
					ClinicalStatus: &fhir.CodeableConcept{
						Text: to.Ptr("active"),
					},
				},
			},
		}

		s := &Service{
			config: Config{
				ParallelBatch: true,
			},
		}

		// Mix of existing and non-existent resources in specific order
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/existing-task-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Task/non-existent-task"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/existing-patient-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Patient/non-existent-patient"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "CarePlan/existing-careplan-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "CarePlan/non-existent-careplan"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/existing-condition-001"}},
				{Request: &fhir.BundleEntryRequest{Method: fhir.HTTPVerbGET, Url: "Condition/non-existent-condition"}},
			},
		}

		actual, err := s.doHandleBatch(httpRequest, requestBundle, fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 8, "Should return exactly 8 entries")

		// Expected results: success, fail, success, fail, success, fail, success, fail
		expectedResults := []struct {
			shouldSucceed bool
			resourceId    string
			statusCode    string
		}{
			{true, "existing-task-001", "200 OK"},
			{false, "", "404 Not Found"},
			{true, "existing-patient-001", "200 OK"},
			{false, "", "404 Not Found"},
			{true, "existing-careplan-001", "200 OK"},
			{false, "", "404 Not Found"},
			{true, "existing-condition-001", "200 OK"},
			{false, "", "404 Not Found"},
		}

		// Verify each entry matches expected result
		for i, expected := range expectedResults {
			entry := actual.Entry[i]
			require.NotNil(t, entry.Response, "Entry %d should have a response", i)
			require.Equal(t, expected.statusCode, entry.Response.Status,
				"Entry %d should have status %s", i, expected.statusCode)

			if expected.shouldSucceed {
				// Should have a resource
				require.NotNil(t, entry.Resource, "Entry %d should have a resource", i)
				var resourceWithId struct {
					Id string `json:"id"`
				}
				require.NoError(t, json.Unmarshal(entry.Resource, &resourceWithId))
				require.Equal(t, expected.resourceId, resourceWithId.Id,
					"Entry %d should have resource ID %s", i, expected.resourceId)
				require.Nil(t, entry.Response.Outcome, "Entry %d should not have an outcome", i)
			} else {
				// Should have an operation outcome
				require.Nil(t, entry.Resource, "Entry %d should not have a resource", i)
				require.NotNil(t, entry.Response.Outcome, "Entry %d should have an outcome", i)
				require.Equal(t, expected.statusCode, entry.Response.Status)
			}
		}

		// Verify specific resource content for successful entries
		var task fhir.Task
		require.NoError(t, json.Unmarshal(actual.Entry[0].Resource, &task))
		require.Equal(t, "existing-task-001", *task.Id)
		require.Equal(t, fhir.TaskStatusDraft, task.Status)

		var patient fhir.Patient
		require.NoError(t, json.Unmarshal(actual.Entry[2].Resource, &patient))
		require.Equal(t, "existing-patient-001", *patient.Id)
		require.True(t, *patient.Active)

		var carePlan fhir.CarePlan
		require.NoError(t, json.Unmarshal(actual.Entry[4].Resource, &carePlan))
		require.Equal(t, "existing-careplan-001", *carePlan.Id)
		require.Equal(t, fhir.CarePlanIntentPlan, carePlan.Intent)

		var condition fhir.Condition
		require.NoError(t, json.Unmarshal(actual.Entry[6].Resource, &condition))
		require.Equal(t, "existing-condition-001", *condition.Id)
		require.Equal(t, "active", *condition.ClinicalStatus.Text)
	})
	t.Run("successful GET request", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
			},
		}
		s := &Service{}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/123",
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 1)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "200 OK", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Resource)
		var task fhir.Task
		require.NoError(t, json.Unmarshal(actual.Entry[0].Resource, &task))
		require.Equal(t, "123", *task.Id)
	})
	t.Run("with query parameters", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
			},
		}
		s := &Service{}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task?_id=123",
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 1)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "200 OK", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Resource)
		require.Equal(t, `{"type":"searchset","entry":[{"resource":{"id":"123","status":"draft","intent":"","resourceType":"Task"}}],"resourceType":"Bundle"}`,
			string(actual.Entry[0].Resource))
	})
	t.Run("'network error' during request", func(t *testing.T) {
		fhirClient := test.StubFHIRClient{}
		fhirClient.Error = errors.New("network error")
		s := &Service{}
		requestBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/123",
					},
				},
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle, &fhirClient)

		require.NoError(t, err)
		require.Len(t, actual.Entry, 1)
		require.NotNil(t, actual.Entry[0].Response)
		require.Equal(t, "502 Bad Gateway", actual.Entry[0].Response.Status)
		require.NotNil(t, actual.Entry[0].Response.Outcome)
		var outcome fhir.OperationOutcome
		require.NoError(t, json.Unmarshal(actual.Entry[0].Response.Outcome, &outcome))
		require.Len(t, outcome.Issue, 1)
		require.Equal(t, fhir.IssueSeverityWarning, outcome.Issue[0].Severity)
		require.Equal(t, "Upstream FHIR server error: network error", *outcome.Issue[0].Details.Text)
	})
}
