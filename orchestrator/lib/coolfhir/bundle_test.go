package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
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
	fhirClient.EXPECT().ReadWithContext(gomock.Any(), "Task/123", gomock.Any()).DoAndReturn(func(_ context.Context, resource string, data *[]byte, opts ...interface{}) error {
		*data, _ = json.Marshal(fhir.Task{
			Id: to.Ptr("123"),
		})
		return nil
	}).AnyTimes()
	fhirClient.EXPECT().SearchWithContext(gomock.Any(), "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
		if len(searchParams["_id"]) > 0 && searchParams["_id"][0] == "123" {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: json.RawMessage(`{"resourceType":"Task","id":"123"}`),
					},
				},
			}
			return nil
		}
		return errors.New("not found")
	}).AnyTimes()
	fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
	ctx := context.Background()

	t.Run("resource read from response.location (relative URL)", func(t *testing.T) {
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Location: to.Ptr("Task/123"),
				Status:   "200",
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, nil, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "Task/123", *actualEntry.Response.Location)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from response.location (absolute URL)", func(t *testing.T) {
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Location: to.Ptr("http://example.com/fhir/Task/123"),
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, nil, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "Task/123", *actualEntry.Response.Location)
	})
	t.Run("resource read from request BundleEntry.request.url, which contains a literal reference (response.location is nil, e.g. PUT in Azure FHIR)", func(t *testing.T) {
		requestEntry := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task/123",
			},
		}
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "200",
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, requestEntry, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from request BundleEntry.request.url, which contains a logical identifier (response.location is nil, e.g. PUT in Azure FHIR)", func(t *testing.T) {
		requestEntry := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task?_id=123",
			},
		}
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "200",
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, requestEntry, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from request BundleEntry.request.url + .ifNoneExist for conditional create on existing resource (Azure FHIR only returns empty response)", func(t *testing.T) {
		// Could be fixed with Prefer HTTP header, but not supported by Azure FHIR: https://github.com/microsoft/fhir-server/issues/2431
		requestEntry := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url:         "Task",
				IfNoneExist: to.Ptr("_id=123"),
			},
		}
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "200",
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, requestEntry, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Equal(t, "123", *actualResult.Id)
		require.NotEmpty(t, actualEntry.Resource)
		require.Equal(t, "200", actualEntry.Response.Status)
	})
	t.Run("resource read from request BundleEntry.request.url, which doesn't contain a local reference nor logical identifier (not supported now)", func(t *testing.T) {
		requestEntry := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task",
			},
		}
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "200",
			},
		}
		_, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, requestEntry, responseEntry, nil)
		require.EqualError(t, err, "failed to determine resource for transaction response bundle entry, see log for more details")
	})
	t.Run("result to unmarshal into is nil", func(t *testing.T) {
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Location: to.Ptr("http://example.com/fhir/Task/123"),
			},
		}
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, nil, responseEntry, nil)
		require.NoError(t, err)
		require.NotNil(t, actualEntry)
	})
}

func TestWithRequestHeaders(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		entry := &fhir.BundleEntry{}
		WithRequestHeaders(map[string][]string{
			IfNoneExistHeader:     {"ifnoneexist"},
			IfMatchHeader:         {"ifmatch"},
			IfNoneMatchHeader:     {"ifnonematch"},
			IfModifiedSinceHeader: {"ifmodifiedsince"},
		})(entry)
		require.Equal(t, "ifnoneexist", *entry.Request.IfNoneExist)
		require.Equal(t, "ifmatch", *entry.Request.IfMatch)
		require.Equal(t, "ifnonematch", *entry.Request.IfNoneMatch)
		require.Equal(t, "ifmodifiedsince", *entry.Request.IfModifiedSince)
	})
	t.Run("not set", func(t *testing.T) {
		entry := &fhir.BundleEntry{}
		WithRequestHeaders(map[string][]string{})(entry)
		assert.Nil(t, entry.Request.IfMatch)
		assert.Nil(t, entry.Request.IfNoneMatch)
		assert.Nil(t, entry.Request.IfModifiedSince)
		assert.Nil(t, entry.Request.IfNoneExist)
	})
}

func TestWithFullUrl(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		entry := &fhir.BundleEntry{}
		WithFullUrl("")(entry)
		assert.Nil(t, entry.FullUrl)
	})
	t.Run("set", func(t *testing.T) {
		entry := &fhir.BundleEntry{}
		WithFullUrl("http://example.com/fhir/Task/123")(entry)
		require.Equal(t, "http://example.com/fhir/Task/123", *entry.FullUrl)
	})
}
