package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"

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
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")

// For most tests we do not need these to exist, mock the calls to the FHIR server so the test doesn't fail
func mockCustomSearchParams(fhirServerMux *http.ServeMux) {
	fhirServerMux.HandleFunc("GET /fhir/metadata", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.CapabilityStatement{})
	})
	fhirServerMux.HandleFunc("POST /fhir/SearchParameter/_search", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Bundle{})
	})
	fhirServerMux.HandleFunc("POST /fhir/SearchParameter", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Bundle{})
	})
	fhirServerMux.HandleFunc("POST /fhir/$reindex", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Bundle{})
	})
}

//func TestService_Proxy(t *testing.T) {
//	// Test that the service registers the /cps URL that proxies to the backing FHIR server
//	// Setup: configure backing FHIR server to which the service proxies
//	fhirServerMux := http.NewServeMux()
//	var capturedQuery url.Values
//	var capturedRequestHeaders http.Header
//	fhirServerMux.HandleFunc("GET /fhir/Success/1", func(writer http.ResponseWriter, request *http.Request) {
//		capturedQuery = request.URL.Query()
//		capturedRequestHeaders = request.Header
//		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{
//			Intent: "order",
//		})
//	})
//	fhirServerMux.HandleFunc("GET /fhir/Fail/1", func(writer http.ResponseWriter, request *http.Request) {
//		coolfhir.WriteOperationOutcomeFromError(context.Background(), coolfhir.BadRequest("Fail"), "oops", writer)
//	})
//	mockCustomSearchParams(fhirServerMux)
//	fhirServer := httptest.NewServer(fhirServerMux)
//	// Setup: create the service
//	messageBroker := messaging.NewMemoryBroker()
//	service, err := New(Config{
//		FHIR: coolfhir.ClientConfig{
//			BaseURL: fhirServer.URL + "/fhir",
//		},
//	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))
//	require.NoError(t, err)
//	// Setup: configure the service to proxy to the backing FHIR server
//	frontServerMux := http.NewServeMux()
//	service.RegisterHandlers(frontServerMux)
//	frontServer := httptest.NewServer(frontServerMux)
//
//	httpClient := frontServer.Client()
//	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")
//
//	t.Run("ok", func(t *testing.T) {
//		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success/1")
//		require.NoError(t, err)
//		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
//		responseData, _ := io.ReadAll(httpResponse.Body)
//		require.JSONEq(t, `{"resourceType":"Task", "intent":"order", "status":"draft"}`, string(responseData))
//		t.Run("caching is allowed", func(t *testing.T) {
//			assert.Equal(t, "must-understand, private", httpResponse.Header.Get("Cache-Control"))
//		})
//	})
//	t.Run("CapabilityStatement", func(t *testing.T) {
//		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/metadata")
//		require.NoError(t, err)
//		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
//		responseData, _ := io.ReadAll(httpResponse.Body)
//		var capabilityStatement fhir.CapabilityStatement
//		err = json.Unmarshal(responseData, &capabilityStatement)
//		require.NoError(t, err)
//		assert.Len(t, capabilityStatement.Rest, 1)
//		assert.Len(t, capabilityStatement.Rest[0].Security.Service, 1)
//	})
//	t.Run("it proxies query parameters", func(t *testing.T) {
//		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success/1?_identifier=foo|bar")
//		require.NoError(t, err)
//		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
//		assert.Equal(t, "foo|bar", capturedQuery.Get("_identifier"))
//	})
//	t.Run("it proxies FHIR HTTP request headers", func(t *testing.T) {
//		// https://build.fhir.org/http.html#Http-Headers
//		httpRequest, _ := http.NewRequest(http.MethodGet, frontServer.URL+"/cps/Success/1", nil)
//		httpRequest.Header.Set("If-None-Exist", "ine")
//		httpRequest.Header.Set("If-Match", "im")
//		httpRequest.Header.Set("If-Modified-Since", "ims")
//		httpRequest.Header.Set("If-None-Match", "inm")
//
//		_, err := httpClient.Do(httpRequest)
//		require.NoError(t, err)
//		assert.Equal(t, "ine", capturedRequestHeaders.Get("If-None-Exist"))
//		assert.Equal(t, "im", capturedRequestHeaders.Get("If-Match"))
//		assert.Equal(t, "ims", capturedRequestHeaders.Get("If-Modified-Since"))
//		assert.Equal(t, "inm", capturedRequestHeaders.Get("If-None-Match"))
//	})
//	t.Run("upstream FHIR server returns FHIR error with operation outcome", func(t *testing.T) {
//		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Fail/1")
//		require.NoError(t, err)
//		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
//		responseData, _ := io.ReadAll(httpResponse.Body)
//		println(string(responseData))
//		require.JSONEq(t, `{
//  "issue": [
//    {
//      "severity": "error",
//      "code": "processing",
//      "diagnostics": "oops failed: Fail"
//    }
//  ],
//  "resourceType": "OperationOutcome"
//}`, string(responseData))
//	})
//}

type OperationOutcomeWithResourceType struct {
	fhir.OperationOutcome
	ResourceType *string `bson:"resourceType" json:"resourceType"`
}

// TestService_ErrorHandling asserts invalid requests return OperationOutcome
func TestService_ErrorHandling(t *testing.T) {
	fhirServerMux := http.NewServeMux()
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: configure the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(
		Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: fhirServer.URL + "/fhir",
			},
		},
		profile.Test(),
		orcaPublicURL.JoinPath("cps"), messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	service.RegisterHandlers(fhirServerMux)
	server := httptest.NewServer(fhirServerMux)

	// Setup: configure the client
	httpClient := server.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(server.Client().Transport, auth.TestPrincipal1, "")

	// Make an invalid call (not providing JSON payload)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/cps/Task", nil)
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/fhir+json")

	httpResponse, err := httpClient.Do(request)
	require.NoError(t, err)

	// Test response
	require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	require.Equal(t, "application/fhir+json", httpResponse.Header.Get("Content-Type"))

	var target OperationOutcomeWithResourceType
	err = json.NewDecoder(httpResponse.Body).Decode(&target)
	require.NoError(t, err)
	require.Equal(t, "OperationOutcome", *target.ResourceType)

	require.NotNil(t, target)
	require.NotEmpty(t, target.Issue)
	require.Equal(t, "CarePlanService/CreateTask failed: invalid fhir.Task: unexpected end of JSON input", *target.Issue[0].Diagnostics)
}

func TestService_ValidationErrorHandling(t *testing.T) {
	fhirServerMux := http.NewServeMux()
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: configure the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(
		Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: fhirServer.URL + "/fhir",
			},
		},
		profile.Test(),
		orcaPublicURL.JoinPath("cps"), messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	service.RegisterHandlers(fhirServerMux)
	server := httptest.NewServer(fhirServerMux)

	// Setup: configure the client
	httpClient := server.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(server.Client().Transport, auth.TestPrincipal1, "")

	var body = `{"meta":{"versionId":"1","lastUpdated":"2025-07-16T09:52:29.238+00:00","source":"#UDij7lTAHXv1rLRt"},"identifier":[{"use":"usual","system":"http://fhir.nl/fhir/NamingSystem/bsn","value":"99999511"}],"name":[{"text":"abv, abv","family":"abv","given":["abv"]}],"telecom":[{"system":"phone","value":"000","use":"home"},{"system":"email","value":"abv","use":"home"}],"gender":"unknown","birthDate":"1980-01-15","address":[{"use":"home","type":"postal","line":["123 Main Street"],"city":"Hometown","state":"State","postalCode":"12345","country":"Country"}],"resourceType":"Patient"}`
	// Make an invalid call (not providing JSON payload)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/cps/Patient", strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/fhir+json")

	httpResponse, err := httpClient.Do(request)
	require.NoError(t, err)

	// Test response
	require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	require.Equal(t, "application/fhir+json", httpResponse.Header.Get("Content-Type"))

	var target OperationOutcomeWithResourceType
	err = json.NewDecoder(httpResponse.Body).Decode(&target)
	require.NoError(t, err)
	require.Equal(t, "OperationOutcome", *target.ResourceType)

	require.NotNil(t, target)
	require.NotEmpty(t, target.Issue)
	require.Equal(t, "patient phone number should be a dutch mobile number", *target.Issue[0].Diagnostics)
	require.Equal(t, "email is invalid", *target.Issue[1].Diagnostics)
}

func TestService_Handle(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	var capturedRequestBody []byte
	fhirServerMux := http.NewServeMux()
	fhirServerMux.HandleFunc("POST /", func(writer http.ResponseWriter, request *http.Request) {
		capturedRequestBody, _ = io.ReadAll(request.Body)
		if strings.Contains(string(capturedRequestBody), "ERROR-NON-FHIR-RESPONSE") {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		} else if strings.Contains(string(capturedRequestBody), "ERROR-NON-SECURITY-OPERATIONOUTCOME") {
			writer.Header().Set("Content-Type", "application/fhir+json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{
  "resourceType": "OperationOutcome",
  "id": "d7ea5021cd736ff022baf52f178dead2",
  "meta": {
    "lastUpdated": "2024-11-20T14:31:31.6676701+00:00"
  },
  "issue": [
    {
      "severity": "error",
      "code": "processing",
      "diagnostics": "Transaction failed on 'PUT' for the requested url '/ServiceRequest'."
    },
    {
      "severity": "error",
      "code": "invalid",
      "diagnostics": "Found result with Id '4eaffb23-02ab-452c-8ef1-b4c9be7b2425', which did not match the provided Id 'e669F4l0Bk3NJpQzoTzVE0opsQR2iWXR41M6FXkeguZo3'."
    }
  ]
}`))
			return
		} else if strings.Contains(string(capturedRequestBody), "ERROR-SECURITY-OPERATIONOUTCOME") {
			writer.Header().Set("Content-Type", "application/fhir+json")
			writer.WriteHeader(http.StatusForbidden)
			json.NewEncoder(writer).Encode(fhir.OperationOutcome{
				Issue: []fhir.OperationOutcomeIssue{
					{
						Severity:    fhir.IssueSeverityError,
						Code:        fhir.IssueTypeSecurity,
						Diagnostics: to.Ptr("You are not authorized to perform this operation"),
					},
				},
			})
			return
		}
		writer.Header().Set("Content-Type", "application/fhir+json")
		writer.WriteHeader(http.StatusOK)
		bundle := map[string]interface{}{
			"resourceType": "Bundle",
			"type":         "transaction",
			"entry": []interface{}{
				map[string]interface{}{
					"response": map[string]interface{}{
						"location": "CarePlan/123",
						"status":   "201 Created",
					},
					"resource": map[string]interface{}{
						"resourceType": "CarePlan",
						"id":           "123",
					},
				},
				map[string]interface{}{
					"response": map[string]interface{}{
						"location": "Task/123",
						"status":   "201 Created",
					},
					"resource": map[string]interface{}{
						"resourceType": "Task",
						"id":           "123",
					},
				},
			},
		}
		json.NewEncoder(writer).Encode(bundle)
	})
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.Test(), orcaPublicURL.JoinPath("cps"), messageBroker, events.NewManager(messageBroker))

	var capturedHeaders []http.Header
	service.handlerProvider = func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		switch method {
		case http.MethodPost:
			switch resourceType {
			case "CarePlan":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("CarePlan"))
						carePlan := fhir.CarePlan{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(carePlan)
						return []*fhir.BundleEntry{result}, []any{&carePlan}, nil
					}, nil
				}
			case "Task":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("Task"))
						task := fhir.Task{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(task)
						return []*fhir.BundleEntry{result}, []any{&task}, nil
					}, nil
				}
			}
		case http.MethodPut:
			switch resourceType {
			case "Task":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("Task"))
						task := fhir.Task{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(task)
						return []*fhir.BundleEntry{result}, []any{&task}, nil
					}, nil
				}
			case "Organization":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					return nil, errors.New("this fails on purpose")
				}
			}
		}
		return nil
	}
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")
	cpsBaseUrl, _ := url.Parse(frontServer.URL)
	fhirClient := fhirclient.New(cpsBaseUrl.JoinPath("cps"), httpClient, nil)

	t.Run("Bundle", func(t *testing.T) {
		t.Run("POST 2 items (CarePlan, Task)", func(t *testing.T) {
			// Create a bundle with 2 Tasks
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
							Url:    "CarePlan",
						},
						Resource: json.RawMessage(`{}`),
					},
					{
						FullUrl: to.Ptr("urn:uuid:task"),
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.NoError(t, err)
			require.Len(t, resultBundle.Entry, 2)
			assert.Equal(t, "CarePlan/123", *resultBundle.Entry[0].Response.Location)
			assert.JSONEq(t, `{"id":"123","status":"draft","intent":"proposal","subject":{},"resourceType":"CarePlan"}`, string(resultBundle.Entry[0].Resource))
			assert.Equal(t, "Task/123", *resultBundle.Entry[1].Response.Location)
			assert.JSONEq(t, `{"id":"123","status":"draft","intent":"","resourceType":"Task"}`, string(resultBundle.Entry[1].Resource))

			t.Run("Bundle.entry.fullUrl is retained", func(t *testing.T) {
				assert.Contains(t, string(capturedRequestBody), `"fullUrl":"urn:uuid:task"`)
			})
		})
		t.Run("PUT 1 item (Task)", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"), fhirclient.ResponseHeaders(hdrs))

			require.NoError(t, err)
			require.Equal(t, 1, *resultBundle.Total)
			require.Equal(t, fhir.BundleTypeTransactionResponse, resultBundle.Type)
			require.Len(t, resultBundle.Entry, 1)
			assert.Equal(t, "Task/123", *resultBundle.Entry[0].Response.Location)
			assert.Equal(t, "application/fhir+json", hdrs.Get("Content-Type"))
			assert.JSONEq(t, `{"id":"123","status":"draft","intent":"","resourceType":"Task"}`, string(resultBundle.Entry[0].Resource))
		})
		t.Run("FHIR HTTP request headers are passed on", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method:          fhir.HTTPVerbPUT,
							Url:             "Task",
							IfNoneExist:     to.Ptr("ifnoneexist"),
							IfMatch:         to.Ptr("ifmatch"),
							IfModifiedSince: to.Ptr("ifmodifiedsince"),
							IfNoneMatch:     to.Ptr("ifnonematch"),
						},
						Resource: json.RawMessage(`{}`),
					},
				},
			}
			capturedHeaders = nil
			err = fhirClient.Create(requestBundle, new(fhir.Bundle), fhirclient.AtPath("/"))

			require.NoError(t, err)
			require.Len(t, capturedHeaders, 1)
			assert.Equal(t, "ifnoneexist", capturedHeaders[0].Get("If-None-Exist"))
			assert.Equal(t, "ifmatch", capturedHeaders[0].Get("If-Match"))
			assert.Equal(t, "ifmodifiedsince", capturedHeaders[0].Get("If-Modified-Since"))
			assert.Equal(t, "ifnonematch", capturedHeaders[0].Get("If-None-Match"))
		})
		t.Run("handler fails (PUT Organization)", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Organization",
						},
						Resource: json.RawMessage(`{}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: bundle.entry[0]: this fails on purpose")
		})
		t.Run("commit fails, FHIR server returns non-FHIR response", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{"intent": "ERROR-NON-FHIR-RESPONSE"}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"), fhirclient.ResponseHeaders(hdrs))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] Bundle failed: upstream FHIR server error")
			assert.Equal(t, "application/fhir+json", hdrs.Get("Content-Type"))
		})
		t.Run("commit fails, FHIR server returns OperationOutcome", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{"intent": "ERROR-NON-SECURITY-OPERATIONOUTCOME"}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"), fhirclient.ResponseHeaders(hdrs))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] Transaction failed on 'PUT' for the requested url '/ServiceRequest'.; [invalid error] Found result with Id '4eaffb23-02ab-452c-8ef1-b4c9be7b2425', which did not match the provided Id 'e669F4l0Bk3NJpQzoTzVE0opsQR2iWXR41M6FXkeguZo3'.")
			assert.Equal(t, "application/fhir+json", hdrs.Get("Content-Type"))
		})
		t.Run("commit fails, FHIR server returns OperationOutcome with security issue, which is sanitized", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{"intent": "ERROR-SECURITY-OPERATIONOUTCOME"}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"), fhirclient.ResponseHeaders(hdrs))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] upstream FHIR server error")
			assert.Equal(t, "application/fhir+json", hdrs.Get("Content-Type"))
		})
		t.Run("GET is disallowed", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbGET,
							Url:    "CarePlan/123",
						},
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: only write operations are supported in Bundle")
		})
		t.Run("entry without request.url", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
						},
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: bundle.entry[0].request.url (entry #) is required")
		})
		t.Run("POST with URL containing too many parts", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
							Url:    "CarePlan/123/456",
						},
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: bundle.entry[0].request.url (entry #) has too many paths")
		})
		t.Run("POST with URL containing a fully qualified url", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
							Url:    "https://example.com/fhir/CarePlan/123",
						},
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: bundle.entry[0].request.url (entry #) must be a relative URL")
		})
	})
	t.Run("Single resources", func(t *testing.T) {
		t.Run("POST/Create", func(t *testing.T) {
			var task fhir.Task

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(task, &task, fhirclient.ResponseHeaders(hdrs))

			require.NoError(t, err)
			assert.Equal(t, "123", *task.Id)
			assert.Equal(t, "application/fhir+json", hdrs.Get("Content-Type"))
		})
		t.Run("PUT/Update", func(t *testing.T) {
			var task fhir.Task
			err = fhirClient.Update("Task/123", task, &task)

			require.NoError(t, err)
			assert.Equal(t, "123", *task.Id)
		})
		t.Run("handler fails (PUT/Update Organization)", func(t *testing.T) {
			var org fhir.Organization

			err = fhirClient.Update("Organization/123", org, &org)

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/UpdateOrganization failed: this fails on purpose")
		})
	})
}

func TestService_validateLiteralReferences(t *testing.T) {
	resourceJson, err := os.ReadFile("testdata/literalreference-ok.json")
	require.NoError(t, err)
	var resource fhir.Task
	err = json.Unmarshal(resourceJson, &resource)
	require.NoError(t, err)

	prof := profile.TestProfile{
		CSD: profile.TestCsdDirectory{Endpoint: "https://example.com/fhir"},
	}

	t.Run("ok", func(t *testing.T) {
		err = validateLiteralReferences(context.Background(), prof, resource)
		require.NoError(t, err)
	})
	t.Run("http:// is not allowed", func(t *testing.T) {
		resource := deep.AlterCopy(resource, func(s *fhir.Task) {
			s.Focus.Reference = to.Ptr("http://example.com")
		})
		err := validateLiteralReferences(context.Background(), prof, resource)
		require.EqualError(t, err, "literal reference is URL with scheme http://, only https:// is allowed (path=focus.reference)")
	})
	t.Run("parent directory traversal isn't allowed", func(t *testing.T) {
		resource := deep.AlterCopy(resource, func(s *fhir.Task) {
			s.Focus.Reference = to.Ptr("https://example.com/fhir/../secret-page")
		})
		err := validateLiteralReferences(context.Background(), prof, resource)
		require.EqualError(t, err, "literal reference is URL with parent path segment '..' (path=focus.reference)")
	})
	t.Run("registered base URL", func(t *testing.T) {
		t.Run("path differs", func(t *testing.T) {
			resource := deep.AlterCopy(resource, func(s *fhir.Task) {
				s.Focus.Reference = to.Ptr("https://example.com/alternate/secret-page")
			})
			err = validateLiteralReferences(context.Background(), prof, resource)
			require.EqualError(t, err, "literal reference is not a child of a registered FHIR base URL (path=focus.reference)")
		})
		t.Run("path differs, check trailing slash normalization", func(t *testing.T) {
			resource := deep.AlterCopy(resource, func(s *fhir.Task) {
				s.Focus.Reference = to.Ptr("https://example.com/fhirPatient/not-allowed")
			})
			err = validateLiteralReferences(context.Background(), prof, resource)
			require.EqualError(t, err, "literal reference is not a child of a registered FHIR base URL (path=focus.reference)")
		})
	})
}

func Test_collectLiteralReferences(t *testing.T) {
	resourceJson, err := os.ReadFile("testdata/literalreference-ok.json")
	require.NoError(t, err)
	var resource any
	err = json.Unmarshal(resourceJson, &resource)
	require.NoError(t, err)
	actualRefs := map[string]string{}
	collectLiteralReferences(resource, nil, actualRefs)

	assert.Equal(t, map[string]string{
		"focus.reference":     "urn:uuid:cps-servicerequest-telemonitoring",
		"for.reference":       "https://example.com/fhir/Patient/1",
		"partOf.#0.reference": "https://example.com/fhir/CarePlan/1",
		"partOf.#1.reference": "https://example.com/fhir/CarePlan/2",
		"partOf.#2.reference": "CarePlan/3",
	}, actualRefs)
}

func TestService_ensureCustomSearchParametersExists(t *testing.T) {
	ctx := context.Background()
	t.Run("parameters are created", func(t *testing.T) {
		fhirClient := test.StubFHIRClient{}
		service := &Service{
			fhirClient: &fhirClient,
		}
		err := service.ensureCustomSearchParametersExists(ctx)
		require.NoError(t, err)
		require.Len(t, fhirClient.CreatedResources["SearchParameter"], 3)
		// First SearchParameter create, rest should be OK
		searchParam := fhirClient.CreatedResources["SearchParameter"][0].(fhir.SearchParameter)
		assert.Equal(t, "CarePlan-subject-identifier", *searchParam.Id)
		// First SearchParameter re-index, rest should be OK
		require.Len(t, fhirClient.CreatedResources["Parameters"], 1)
		// Split parameters by comma, should match the created resources
		searchParamReindex := fhirClient.CreatedResources["Parameters"][0].(fhir.Parameters)
		params := strings.Split(*searchParamReindex.Parameter[0].ValueString, ",")
		require.Len(t, params, len(fhirClient.CreatedResources["SearchParameter"]))
		for _, sp := range fhirClient.CreatedResources["SearchParameter"] {
			assert.Contains(t, params, sp.(fhir.SearchParameter).Url)
		}
	})
	t.Run("parameters exist", func(t *testing.T) {
		fhirClient := test.StubFHIRClient{
			Resources: []any{
				fhir.SearchParameter{
					Url: "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier",
				},
				fhir.SearchParameter{
					Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json",
				},
				fhir.SearchParameter{
					Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json",
				},
			},
			Metadata: fhir.CapabilityStatement{
				Rest: []fhir.CapabilityStatementRest{
					{
						Resource: []fhir.CapabilityStatementRestResource{
							{
								SearchParam: []fhir.CapabilityStatementRestResourceSearchParam{
									{
										Definition: to.Ptr("http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier"),
									},
									{
										Definition: to.Ptr("http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json"),
									},
									{
										Definition: to.Ptr("http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json"),
									},
								},
							},
						},
					},
				},
			},
		}
		service := &Service{
			fhirClient: &fhirClient,
		}
		err := service.ensureCustomSearchParametersExists(ctx)
		require.NoError(t, err)
		require.Empty(t, fhirClient.CreatedResources)
	})
	t.Run("Azure FHIR: parameter exists, but needs to be re-indexed (all)", func(t *testing.T) {
		t.Log("SearchParameter exists, but doesn't show up in CapabilityStatement. On Azure FHIR, this indicates the parameter needs to be re-indexed.")

		resources := []any{
			fhir.SearchParameter{
				Url: "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier",
			},
			fhir.SearchParameter{
				Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json",
			},
			fhir.SearchParameter{
				Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json",
			},
		}

		fhirClient := test.StubFHIRClient{
			Resources: resources,
		}
		service := &Service{
			fhirClient: &fhirClient,
		}
		err := service.ensureCustomSearchParametersExists(ctx)
		require.NoError(t, err)
		// SearchParameter isn't created, but is re-indexed.
		require.Empty(t, fhirClient.CreatedResources["SearchParameter"])
		require.Len(t, fhirClient.CreatedResources["Parameters"], 1)
		// Split parameters by comma, should match the created resources
		searchParamReindex := fhirClient.CreatedResources["Parameters"][0].(fhir.Parameters)
		params := strings.Split(*searchParamReindex.Parameter[0].ValueString, ",")
		// Assert the length of params is equal to the amount of SearchParameters in the FHIR client, minus 1 because of the newly created parameter
		require.Len(t, params, len(resources))
		for _, sp := range resources {
			assert.Contains(t, params, sp.(fhir.SearchParameter).Url)
		}
	})
	t.Run("Azure FHIR: parameter exists, but needs to be re-indexed (only one)", func(t *testing.T) {
		t.Log("SearchParameter exists, but doesn't show up in CapabilityStatement. On Azure FHIR, this indicates the parameter needs to be re-indexed.")

		resources := []any{
			fhir.SearchParameter{
				Url: "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier",
			},
			fhir.SearchParameter{
				Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json",
			},
			fhir.SearchParameter{
				Url: "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json",
			},
		}

		fhirClient := test.StubFHIRClient{
			Resources: resources,
			Metadata: fhir.CapabilityStatement{
				Rest: []fhir.CapabilityStatementRest{
					{
						Resource: []fhir.CapabilityStatementRestResource{
							{
								SearchParam: []fhir.CapabilityStatementRestResourceSearchParam{
									{
										Definition: to.Ptr("http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json"),
									},
									{
										Definition: to.Ptr("http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json"),
									},
								},
							},
						},
					},
				},
			},
		}
		service := &Service{
			fhirClient: &fhirClient,
		}
		err := service.ensureCustomSearchParametersExists(ctx)
		require.NoError(t, err)
		// SearchParameter isn't created, but is re-indexed.
		require.Empty(t, fhirClient.CreatedResources["SearchParameter"])
		require.Len(t, fhirClient.CreatedResources["Parameters"], 1)
		// Split parameters by comma, should match the created resources
		searchParamReindex := fhirClient.CreatedResources["Parameters"][0].(fhir.Parameters)
		params := strings.Split(*searchParamReindex.Parameter[0].ValueString, ",")
		// We only expect 1 param that needs to be re-indexed
		require.Len(t, params, 1)
		assert.Equal(t, "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier", params[0])
	})
}

func TestService_validateSearchRequest(t *testing.T) {
	fhirServerMux := http.NewServeMux()
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.Test(), orcaPublicURL.JoinPath("cps"), messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	t.Run("invalid content type - fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", nil)
		req.Header.Set("Content-Type", "application/json")

		err := service.validateSearchRequest(req)

		assert.EqualError(t, err, "Content-Type must be 'application/x-www-form-urlencoded'")
	})

	t.Run("invalid encoded body parameters JSON - fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader(`{"invalid":"param"}`))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.EqualError(t, err, "Invalid encoded body parameters")
	})

	t.Run("invalid encoded body parameters - fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader("valid=param&invalid"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.EqualError(t, err, "Invalid encoded body parameters")
	})

	t.Run("valid encoded body parameters - succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader("valid=param"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.NoError(t, err)
	})

	t.Run("valid encoded body parameters with multiple values - succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader("valid=param1&valid=param2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.NoError(t, err)
	})

	t.Run("valid encoded body parameters with charset - succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader("valid=param1&valid=param2"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")

		err := service.validateSearchRequest(req)

		assert.NoError(t, err)
	})

	t.Run("valid encoded body parameters with empty values - succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", strings.NewReader("valid1=&valid2="))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.NoError(t, err)
	})

	t.Run("empty body - succeeds", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/cps/CarePlan/_search", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		err := service.validateSearchRequest(req)

		assert.NoError(t, err)
	})
}

func TestService_notifySubscribers(t *testing.T) {
	t.Run("CareTeam causes notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		subscriptionManager := subscriptions.NewMockManager(ctrl)
		subscriptionManager.EXPECT().Notify(gomock.Any(), gomock.Any())
		s := &Service{
			subscriptionManager: subscriptionManager,
		}
		s.notifySubscribers(context.Background(), &fhir.CareTeam{})
	})
	t.Run("CarePlan causes notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		subscriptionManager := subscriptions.NewMockManager(ctrl)
		subscriptionManager.EXPECT().Notify(gomock.Any(), gomock.Any())
		s := &Service{
			subscriptionManager: subscriptionManager,
		}
		s.notifySubscribers(context.Background(), &fhir.CarePlan{})
	})
	t.Run("Task causes notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		subscriptionManager := subscriptions.NewMockManager(ctrl)
		subscriptionManager.EXPECT().Notify(gomock.Any(), gomock.Any())
		s := &Service{
			subscriptionManager: subscriptionManager,
		}
		s.notifySubscribers(context.Background(), &fhir.Task{})
	})
	t.Run("Other resource type does not cause notification", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		subscriptionManager := subscriptions.NewMockManager(ctrl)
		s := &Service{
			subscriptionManager: subscriptionManager,
		}
		s.notifySubscribers(context.Background(), &fhir.ActivityDefinition{})
	})
}

func TestService_Tracing(t *testing.T) {
	// Setup in-memory tracer for testing
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(
		trace.WithSyncer(exporter),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(context.Background())

	// Setup test service
	fhirServerMux := http.NewServeMux()
	mockCustomSearchParams(fhirServerMux)
	fhirServerMux.HandleFunc("POST /fhir/", func(writer http.ResponseWriter, request *http.Request) {
		// Mock FHIR transaction response
		bundle := fhir.Bundle{
			Type: fhir.BundleTypeTransactionResponse,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Status:   "201 Created",
						Location: to.Ptr("Patient/123"),
					},
					Resource: json.RawMessage(`{"resourceType":"Patient","id":"123"}`),
				},
			},
		}
		coolfhir.SendResponse(writer, http.StatusOK, bundle)
	})
	fhirServer := httptest.NewServer(fhirServerMux)

	messageBroker := messaging.NewMemoryBroker()
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("handleModification creates span with correct attributes", func(t *testing.T) {
		// Clear previous spans
		exporter.Reset()

		// Make a POST request to create a Patient
		patientData, _ := json.Marshal(fhir.Patient{
			Name: []fhir.HumanName{
				{
					Family: to.Ptr("Doe"),
					Given:  []string{"John"},
				},
			},
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://example.com/fhir/identifier"),
					Value:  to.Ptr("12345"),
				},
			},
			Telecom: telecom,
		})
		httpResponse, err := httpClient.Post(frontServer.URL+"/cps/Patient", "application/json", strings.NewReader(string(patientData)))
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, httpResponse.StatusCode)

		// Verify span was created
		spans := exporter.GetSpans()
		// This span is created by the FHIR client, not by ORCA
		assertTracingSpan(t, exporter, "fhir.post /fhir/", []attribute.KeyValue{}, codes.Unset)
		assertTracingSpan(t, exporter, "commitTransaction", []attribute.KeyValue{
			attribute.String("fhir.bundle.type", "transaction"),
			attribute.Int("fhir.bundle.entry_count", 2),
			attribute.Int("fhir.transaction.result_entries", 1),
		}, codes.Ok)
		assertTracingSpan(t, exporter, "handleModification", []attribute.KeyValue{
			attribute.String("http.method", "POST"),
			attribute.String("fhir.resource_type", "Patient"),
			attribute.String("operation.name", "CarePlanService/CreatePatient"),
		}, codes.Ok)

		assertSpansBelongToSameTrace(t, spans)
	})

	t.Run("handleModification records error in span", func(t *testing.T) {
		// Clear previous spans
		exporter.Reset()

		// Setup FHIR server to return an error
		fhirServerMux.HandleFunc("POST /fhir/error", func(writer http.ResponseWriter, request *http.Request) {
			coolfhir.WriteOperationOutcomeFromError(context.Background(),
				coolfhir.BadRequest("test error"), "test", writer)
		})

		// Make a request that will cause an error (invalid JSON)
		httpResponse, err := httpClient.Post(frontServer.URL+"/cps/Patient", "application/json", strings.NewReader("invalid json"))
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)

		assertTracingSpan(t, exporter, "handleModification", []attribute.KeyValue{
			attribute.String("http.method", "POST"),
			attribute.String("fhir.resource_type", "Patient"),
			attribute.String("operation.name", "CarePlanService/CreatePatient"),
		}, codes.Error)
	})
}

func assertTracingSpan(t *testing.T, exporter *tracetest.InMemoryExporter, expectedSpanName string, expectedAttributes []attribute.KeyValue, expectedStatusCode codes.Code) {
	t.Helper()

	spans := exporter.GetSpans()

	var operationSpan *tracetest.SpanStub
	for _, span := range spans {
		if span.Name == expectedSpanName {
			operationSpan = &span
			break
		}
	}
	require.NotNil(t, operationSpan, "Expected %s span", expectedSpanName)

	attributes := operationSpan.Attributes
	for _, expectedAttr := range expectedAttributes {
		found := false
		for _, actualAttr := range attributes {
			if actualAttr.Key == expectedAttr.Key && actualAttr.Value.AsString() == expectedAttr.Value.AsString() {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected attribute %s=%s not found in span attributes", expectedAttr.Key, expectedAttr.Value.AsString())
	}
	assert.Equal(t, expectedStatusCode, operationSpan.Status.Code)
}

func assertSpansBelongToSameTrace(t *testing.T, spans []tracetest.SpanStub) {
	t.Helper()

	if len(spans) == 0 {
		t.Fatal("No spans provided to verify")
		return
	}

	expectedTraceID := spans[0].SpanContext.TraceID()
	if !expectedTraceID.IsValid() {
		t.Fatal("First span has invalid trace ID")
		return
	}

	for i, span := range spans {
		spanTraceID := span.SpanContext.TraceID()
		if !spanTraceID.IsValid() {
			t.Errorf("Span %d has invalid trace ID", i)
			continue
		}

		if spanTraceID.String() != expectedTraceID.String() {
			t.Errorf("Span %d has different trace ID: expected %s, got %s",
				i, expectedTraceID.String(), spanTraceID.String())
		}
	}
}
