package coolfhir

import (
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"go.uber.org/mock/gomock"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestTransactionBuilder(t *testing.T) {
	tx := Transaction().
		Create(fhir.Task{
			Id: to.Ptr("task1"),
		}).
		Create(fhir.Task{
			Id: to.Ptr("task2"),
		}).
		Bundle()

	require.Equal(t, fhir.BundleTypeTransaction, tx.Type)
	require.Len(t, tx.Entry, 2)

	var task1 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[0].Resource, &task1))
	require.Equal(t, "task1", task1["id"])
	var task2 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[1].Resource, &task2))
	require.Equal(t, "task2", task2["id"])
}

func TestFetchBundleEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	fhirClient := mock.NewMockClient(ctrl)
	fhirClient.EXPECT().Read("Task/123", gomock.Any(), gomock.Any()).DoAndReturn(func(resource string, data *[]byte, opts ...interface{}) error {
		*data, _ = json.Marshal(fhir.Task{
			Id: to.Ptr("123"),
		})
		return nil
	}).AnyTimes()
	fhirClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(resource string, data *[]byte, opts ...interface{}) error {
		httpRequest := httptest.NewRequest("GET", "http://example.com/fhir/Task", nil)
		opts[0].(fhirclient.PreRequestOption)(fhirClient, httpRequest)
		if httpRequest.URL.Query()["_id"][0] == "123" {
			*data, _ = json.Marshal(fhir.Task{
				Id: to.Ptr("123"),
			})
			return nil
		}
		return errors.New("not found")
	}).AnyTimes()
	fhirBaseUrl, _ := url.Parse("http://example.com/fhir")

	t.Run("resource read from response.location (relative URL)", func(t *testing.T) {
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("Task/123"),
						Status:   "200",
					},
				},
			},
		}
		var actualResult fhir.Task
		actualEntry, err := FetchBundleEntry(fhirClient, fhirBaseUrl, nil, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "Task/123", *actualEntry.Response.Location)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from response.location (absolute URL)", func(t *testing.T) {
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("http://example.com/fhir/Task/123"),
					},
				},
			},
		}
		var actualResult fhir.Task
		actualEntry, err := FetchBundleEntry(fhirClient, fhirBaseUrl, nil, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "Task/123", *actualEntry.Response.Location)
	})
	t.Run("resource read from request BundleEntry.request.url, which contains a literal reference (response.location is nil, e.g. PUT in Azure FHIR)", func(t *testing.T) {
		request := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task/123",
			},
		}
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Status: "200",
					},
				},
			},
		}
		var actualResult fhir.Task
		actualEntry, err := FetchBundleEntry(fhirClient, fhirBaseUrl, request, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from request BundleEntry.request.url, which contains a logical identifier (response.location is nil, e.g. PUT in Azure FHIR)", func(t *testing.T) {
		request := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task?_id=123",
			},
		}
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Status: "200",
					},
				},
			},
		}
		var actualResult fhir.Task
		actualEntry, err := FetchBundleEntry(fhirClient, fhirBaseUrl, request, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from request BundleEntry.request.url, which doesn't contain a local reference nor logical identifier (not supported now)", func(t *testing.T) {
		request := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task",
			},
		}
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Status: "200",
					},
				},
			},
		}
		_, err := FetchBundleEntry(fhirClient, fhirBaseUrl, request, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, nil)
		require.EqualError(t, err, "failed to determine resource path for entry 0, see log for more details")
	})
	t.Run("result to unmarshal into is nil", func(t *testing.T) {
		response := &fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("http://example.com/fhir/Task/123"),
					},
				},
			},
		}
		actualEntry, err := FetchBundleEntry(fhirClient, fhirBaseUrl, nil, response, func(_ int, _ fhir.BundleEntry) bool {
			return true
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, actualEntry)
	})
}
