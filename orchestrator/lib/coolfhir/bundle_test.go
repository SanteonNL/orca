package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
	t.Run("delete with 204 No Content", func(t *testing.T) {
		requestEntry := &fhir.BundleEntry{
			Request: &fhir.BundleEntryRequest{
				Url: "Task",
			},
		}
		responseEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "204 No Content",
			},
		}
		var actualResult fhir.Task
		actualEntry, err := NormalizeTransactionBundleResponseEntry(ctx, fhirClient, fhirBaseUrl, requestEntry, responseEntry, &actualResult)
		require.NoError(t, err)
		require.Nil(t, actualEntry.Resource)
		require.Equal(t, "204 No Content", actualEntry.Response.Status)
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

func TestBundleBuilder_Append(t *testing.T) {
	// Setup a fixed time for testing
	originalNowFunc := nowFunc
	defer func() { nowFunc = originalNowFunc }()
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return fixedTime }

	t.Run("with pre and post options", func(t *testing.T) {
		// Create a test resource
		patient := fhir.Patient{
			Id: to.Ptr("test-patient"),
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Doe"),
					Given:  []string{"John"},
				},
			},
		}

		// Create pre and post options for testing
		var preOptionCalled, postOptionCalled bool
		preOption := BundleEntryPreOption(func(entry *fhir.BundleEntry) {
			preOptionCalled = true
			entry.FullUrl = to.NilString("Patient/test-patient")
		})
		postOption := BundleEntryPostOption(func(entry *fhir.BundleEntry) {
			postOptionCalled = true
			// Verify the pre-option was applied
			assert.Equal(t, "Patient/test-patient", *entry.FullUrl)
		})

		// Create a bundle and append the resource with options
		bundle := Transaction()
		bundle.Append(patient, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPOST,
			Url:    "Patient",
		}, nil, preOption, postOption)

		// Verify options were called
		assert.True(t, preOptionCalled, "Pre-option should have been called")
		assert.True(t, postOptionCalled, "Post-option should have been called")

		// Verify bundle content
		assert.Len(t, bundle.Entry, 1)
		assert.Equal(t, "Patient/test-patient", *bundle.Entry[0].FullUrl)
	})

	t.Run("WithAuditEvent option", func(t *testing.T) {
		ctx := context.Background()

		// Create a test resource
		patient := fhir.Patient{
			Id: to.Ptr("test-patient"),
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Doe"),
					Given:  []string{"John"},
				},
			},
		}

		// Create bundle
		bundle := Transaction()

		// Setup audit event info
		auditInfo := AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Reference: to.Ptr("Practitioner/test-practitioner"),
			},
			Observer: fhir.Identifier{
				System: to.Ptr("http://example.org/systems/identifier"),
				Value:  to.Ptr("test-device"),
			},
			Action: fhir.AuditEventActionC,
		}

		// Add entry with audit event
		bundle.Append(patient, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPOST,
			Url:    "Patient",
		}, nil, WithFullUrl("Patient/test-patient"), WithAuditEvent(ctx, bundle, auditInfo))

		// Verify bundle content
		require.Len(t, bundle.Entry, 2)

		// First entry should be the patient
		assert.Equal(t, "Patient/test-patient", *bundle.Entry[0].FullUrl)

		// Second entry should be the audit event
		var auditEvent fhir.AuditEvent
		err := json.Unmarshal(bundle.Entry[1].Resource, &auditEvent)
		require.NoError(t, err)

		// Verify audit event content
		assert.Equal(t, "rest", *auditEvent.Type.Code)
		assert.Equal(t, "create", *auditEvent.Subtype[0].Code)
		assert.Equal(t, fhir.AuditEventActionC, *auditEvent.Action)
		assert.Equal(t, fixedTime.Format(time.RFC3339), auditEvent.Recorded)
		assert.Equal(t, "Practitioner/test-practitioner", *auditEvent.Agent[0].Who.Reference)
		assert.Equal(t, "test-device", *auditEvent.Source.Observer.Identifier.Value)
		assert.Equal(t, "Patient/test-patient", *auditEvent.Entity[0].What.Reference)
	})

	t.Run("WithAuditEvent with query parameters", func(t *testing.T) {
		ctx := context.Background()

		// Create a test resource
		patient := fhir.Patient{
			Id: to.Ptr("test-patient"),
		}

		// Create bundle
		bundle := Transaction()

		// Setup audit event info with query parameters
		queryParams := url.Values{}
		queryParams.Add("name", "Doe")
		queryParams.Add("gender", "male")

		auditInfo := AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Reference: to.Ptr("Practitioner/test-practitioner"),
			},
			Observer: fhir.Identifier{
				Value: to.Ptr("test-device"),
			},
			Action:      fhir.AuditEventActionR,
			QueryParams: queryParams,
		}

		// Add entry with audit event
		bundle.Append(patient, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbGET,
			Url:    "Patient?name=Doe&gender=male",
		}, nil, WithFullUrl("Patient/test-patient"), WithAuditEvent(ctx, bundle, auditInfo))

		// Verify bundle content
		require.Len(t, bundle.Entry, 2)

		// Second entry should be the audit event
		var auditEvent fhir.AuditEvent
		err := json.Unmarshal(bundle.Entry[1].Resource, &auditEvent)
		require.NoError(t, err)

		// Verify audit event content
		assert.Equal(t, "read", *auditEvent.Subtype[0].Code)
		assert.Equal(t, fhir.AuditEventActionR, *auditEvent.Action)

		// Verify query parameters in the audit event
		require.Len(t, auditEvent.Entity, 2)

		// Find the query parameters entity
		var queryEntity *fhir.AuditEventEntity
		for i := range auditEvent.Entity {
			if auditEvent.Entity[i].Type != nil && *auditEvent.Entity[i].Type.Code == "2" {
				queryEntity = &auditEvent.Entity[i]
				break
			}
		}

		require.NotNil(t, queryEntity, "Query parameters entity should exist")
		assert.Equal(t, "Query Parameters", *queryEntity.Type.Display)

		// Verify the query parameter details
		paramMap := make(map[string]string)
		for _, detail := range queryEntity.Detail {
			paramMap[detail.Type] = *detail.ValueString
		}

		assert.Equal(t, "Doe", paramMap["name"])
		assert.Equal(t, "male", paramMap["gender"])
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

	// Create a test resource
	patient := fhir.Patient{
		Id: to.Ptr("test-patient"),
	}

	// Create HTTP headers
	headers := http.Header{}
	headers.Set(IfNoneExistHeader, "identifier=123")
	headers.Set(IfMatchHeader, "W/\"1\"")
	headers.Set(IfNoneMatchHeader, "W/\"2\"")
	headers.Set(IfModifiedSinceHeader, "Mon, 01 Jan 2023 12:00:00 GMT")

	// Create a bundle and append the resource with headers
	bundle := Transaction()
	bundle.Append(patient, &fhir.BundleEntryRequest{
		Method: fhir.HTTPVerbPOST,
		Url:    "Patient",
	}, nil, WithRequestHeaders(headers))

	// Verify headers were applied to the request
	require.Len(t, bundle.Entry, 1)
	require.NotNil(t, bundle.Entry[0].Request)
	assert.Equal(t, "identifier=123", *bundle.Entry[0].Request.IfNoneExist)
	assert.Equal(t, "W/\"1\"", *bundle.Entry[0].Request.IfMatch)
	assert.Equal(t, "W/\"2\"", *bundle.Entry[0].Request.IfNoneMatch)
	assert.Equal(t, "Mon, 01 Jan 2023 12:00:00 GMT", *bundle.Entry[0].Request.IfModifiedSince)

	// Test HeadersFromBundleEntryRequest
	extractedHeaders := HeadersFromBundleEntryRequest(bundle.Entry[0].Request)
	assert.Equal(t, "identifier=123", extractedHeaders.Get(IfNoneExistHeader))
	assert.Equal(t, "W/\"1\"", extractedHeaders.Get(IfMatchHeader))
	assert.Equal(t, "W/\"2\"", extractedHeaders.Get(IfNoneMatchHeader))
	assert.Equal(t, "Mon, 01 Jan 2023 12:00:00 GMT", extractedHeaders.Get(IfModifiedSinceHeader))
}

func TestFailedBundleEntry(t *testing.T) {
	ctx := context.Background()

	// Create a malformed resource that will fail to unmarshal
	malformedResource := struct {
		Invalid string `json:"invalid"`
	}{
		Invalid: "data",
	}

	t.Run("missing resource type and ID", func(t *testing.T) {
		// Create bundle
		bundle := Transaction()

		// Setup audit event info
		auditInfo := AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Reference: to.Ptr("Practitioner/test-practitioner"),
			},
			Observer: fhir.Identifier{
				Value: to.Ptr("test-device"),
			},
			Action: fhir.AuditEventActionC,
		}

		// Add entry with audit event that will fail due to missing resource type
		bundle.Append(malformedResource, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPOST,
			Url:    "malformed",
		}, nil, WithAuditEvent(ctx, bundle, auditInfo))

		// Verify bundle contains the original entry and a failedBundleEntry
		require.Len(t, bundle.Entry, 2)

		var resource FailedBundleEntry
		err := json.Unmarshal(bundle.Entry[1].Resource, &resource)
		require.NoError(t, err)

		// The resource type should be FailedBundleEntry
		assert.Equal(t, "", resource.ResourceType)
		assert.Equal(t, "", resource.ID)
		assert.Equal(t, fhir.HTTPVerbPOST, resource.Method)
		assert.Contains(t, resource.Error, "Unable to create proper reference for audit event, missing both FullUrl and resource ID")
	})
}
