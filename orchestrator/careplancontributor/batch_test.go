package careplancontributor

import (
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"testing"
)

func TestService_handleBatch(t *testing.T) {
	httpRequest, _ := http.NewRequest(http.MethodGet, "/", nil)
	httpRequest.Header.Add("X-Scp-Context", "valid")
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
		s := &Service{
			ehrFhirClient: &fhirClient,
		}
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
		actual, err := s.doHandleBatch(httpRequest, requestBundle)

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
			ehrFhirClient: &test.StubFHIRClient{},
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
		actual, err := s.doHandleBatch(httpRequest, requestBundle)

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
	t.Run("successful GET request", func(t *testing.T) {
		s := &Service{
			ehrFhirClient: &test.StubFHIRClient{
				Resources: []any{
					fhir.Task{
						Id: to.Ptr("123"),
					},
				},
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
			},
		}
		actual, err := s.doHandleBatch(httpRequest, requestBundle)

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
		s := &Service{
			ehrFhirClient: &test.StubFHIRClient{
				Resources: []any{
					fhir.Task{
						Id: to.Ptr("123"),
					},
				},
			},
		}
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
		actual, err := s.doHandleBatch(httpRequest, requestBundle)

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
		s := &Service{
			ehrFhirClient: &fhirClient,
		}
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
		actual, err := s.doHandleBatch(httpRequest, requestBundle)

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
