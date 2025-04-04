package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/stretchr/testify/assert"
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

func TestService_Proxy(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	var capturedQuery url.Values
	var capturedRequestHeaders http.Header
	fhirServerMux.HandleFunc("GET /fhir/Success/1", func(writer http.ResponseWriter, request *http.Request) {
		capturedQuery = request.URL.Query()
		capturedRequestHeaders = request.Header
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{
			Intent: "order",
		})
	})
	fhirServerMux.HandleFunc("GET /fhir/Fail/1", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.WriteOperationOutcomeFromError(context.Background(), coolfhir.BadRequest("Fail"), "oops", writer)
	})
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(Config{
		AllowUnmanagedFHIROperations: true,
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.policyAgent = NewMockPolicyMiddleware()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("ok", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success/1")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		require.JSONEq(t, `{"resourceType":"Task", "intent":"order", "status":"draft"}`, string(responseData))
		t.Run("caching is allowed", func(t *testing.T) {
			assert.Equal(t, "must-understand, private", httpResponse.Header.Get("Cache-Control"))
		})
	})
	t.Run("CapabilityStatement", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/metadata")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		var capabilityStatement fhir.CapabilityStatement
		err = json.Unmarshal(responseData, &capabilityStatement)
		require.NoError(t, err)
		assert.Len(t, capabilityStatement.Rest, 1)
		assert.Len(t, capabilityStatement.Rest[0].Security.Service, 1)
	})
	t.Run("it proxies query parameters", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success/1?_identifier=foo|bar")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "foo|bar", capturedQuery.Get("_identifier"))
	})
	t.Run("it proxies FHIR HTTP request headers", func(t *testing.T) {
		// https://build.fhir.org/http.html#Http-Headers
		httpRequest, _ := http.NewRequest(http.MethodGet, frontServer.URL+"/cps/Success/1", nil)
		httpRequest.Header.Set("If-None-Exist", "ine")
		httpRequest.Header.Set("If-Match", "im")
		httpRequest.Header.Set("If-Modified-Since", "ims")
		httpRequest.Header.Set("If-None-Match", "inm")

		_, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		assert.Equal(t, "ine", capturedRequestHeaders.Get("If-None-Exist"))
		assert.Equal(t, "im", capturedRequestHeaders.Get("If-Match"))
		assert.Equal(t, "ims", capturedRequestHeaders.Get("If-Modified-Since"))
		assert.Equal(t, "inm", capturedRequestHeaders.Get("If-None-Match"))
	})
	t.Run("upstream FHIR server returns FHIR error with operation outcome", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Fail/1")
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		println(string(responseData))
		require.JSONEq(t, `{
  "issue": [
    {
      "severity": "error",
      "code": "processing",
      "diagnostics": "oops failed: Fail"
    }
  ],
  "resourceType": "OperationOutcome"
}`, string(responseData))
	})
	t.Run("disallowed unmanaged FHIR operation", func(t *testing.T) {
		messageBroker := messaging.NewMemoryBroker()
		service, err := New(Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: fhirServer.URL + "/fhir",
			},
		}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))
		require.NoError(t, err)
		frontServerMux := http.NewServeMux()
		service.policyAgent = NewMockPolicyMiddleware()
		service.RegisterHandlers(frontServerMux)
		frontServer := httptest.NewServer(frontServerMux)

		httpClient := frontServer.Client()
		httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Anything/1")
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		require.Contains(t, string(responseData), "FHIR operation not allowed")
	})
}

func TestService_Proxy_AllowUnmanagedOperations(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/SomeResource/1", func(writer http.ResponseWriter, request *http.Request) {
		capturedHost = request.Host
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{})
	})
	var capturedBundle fhir.Bundle
	fhirServerMux.HandleFunc("POST /fhir/", func(writer http.ResponseWriter, request *http.Request) {
		if err := json.NewDecoder(request.Body).Decode(&capturedBundle); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Status: "204 No Content",
					},
				},
			},
		})
	})
	var capturedBody []byte
	fhirServerMux.HandleFunc("POST /fhir/SomeResource/_search", func(writer http.ResponseWriter, request *http.Request) {
		capturedHost = request.Host
		capturedBody, _ = io.ReadAll(request.Body)
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Bundle{})
	})
	mockCustomSearchParams(fhirServerMux)
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	// Setup: create the service
	messageBroker := messaging.NewMemoryBroker()
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
		AllowUnmanagedFHIROperations: true,
	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))

	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.policyAgent = NewMockPolicyMiddleware()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("read", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/SomeResource/1")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, fhirServerURL.Host, capturedHost)
	})

	// Test POST edge cases
	t.Run("search", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			httpResponse, err := httpClient.Post(frontServer.URL+"/cps/SomeResource/_search", "application/x-www-form-urlencoded", strings.NewReader(`identifier=foo`))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			require.Equal(t, fhirServerURL.Host, capturedHost)
			require.Equal(t, "identifier=foo", string(capturedBody))
		})
		t.Run("invalid path (_search must go directly after the resource)", func(t *testing.T) {
			httpResponse, err := httpClient.Post(frontServer.URL+"/cps/SomeResource/1/_search", "application/fhir+json", nil)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			responseBody, _ := io.ReadAll(httpResponse.Body)
			assert.Contains(t, string(responseBody), "invalid path")
		})
	})
	t.Run("delete", func(t *testing.T) {
		t.Run("by search parameter", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodDelete, frontServer.URL+"/cps/SomeResource?identifier=foo", nil)
			httpResponse, err := httpClient.Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, httpResponse.StatusCode)
			require.Equal(t, "SomeResource?identifier=foo", capturedBundle.Entry[0].Request.Url)
			require.Equal(t, fhir.HTTPVerbDELETE, capturedBundle.Entry[0].Request.Method)
		})
		t.Run("by ID", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodDelete, frontServer.URL+"/cps/SomeResource/1", nil)
			httpResponse, err := httpClient.Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, httpResponse.StatusCode)
			require.Equal(t, "SomeResource/1", capturedBundle.Entry[0].Request.Url)
			require.Equal(t, fhir.HTTPVerbDELETE, capturedBundle.Entry[0].Request.Method)
		})
	})
}

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
		orcaPublicURL, messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	service.policyAgent = NewMockPolicyMiddleware()
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

func TestService_DefaultOperationHandler(t *testing.T) {
	t.Run("handles unmanaged FHIR operations - allow unmanaged operations", func(t *testing.T) {
		// For now, we have a flag that can be enabled in config that will allow unmanaged FHIR operations. This defaults to false and should not be enabled in test or prod environments
		tx := coolfhir.Transaction()
		// The unmanaged operation handler reads the resource to return from the result Bundle,
		// from the same index as the resource it added to the transaction Bundle. To test this,
		// we make sure there's 2 other resources in the Bundle.
		tx.Create(fhir.Task{})
		txResultBundle := fhir.Bundle{
			Type: fhir.BundleTypeTransaction,
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("Task/456"),
						Status:   "201 Created",
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("ServiceRequest/123"),
						Status:   "201 Created",
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("Task/789"),
						Status:   "201 Created",
					},
				},
			},
		}
		expectedServiceRequest := fhir.ServiceRequest{
			Id: to.Ptr("123"),
		}
		expectedServiceRequestJson, _ := json.Marshal(expectedServiceRequest)
		request := FHIRHandlerRequest{
			ResourcePath: "ServiceRequest",
			HttpMethod:   http.MethodPost,
			Context:      context.Background(),
		}
		ctrl := gomock.NewController(t)
		fhirClient := mock.NewMockClient(ctrl)
		fhirClient.EXPECT().ReadWithContext(gomock.Any(), "ServiceRequest/123", gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ string, resultResource interface{}, opts ...fhirclient.Option) error {
				reflect.ValueOf(resultResource).Elem().Set(reflect.ValueOf(expectedServiceRequestJson))
				return nil
			})
		fhirBaseUrl, _ := url.Parse("http://example.com")
		service := Service{
			fhirClient:                   fhirClient,
			allowUnmanagedFHIROperations: true,
			fhirURL:                      fhirBaseUrl,
		}

		resultHandler, err := service.handleUnmanagedOperation(request, tx)
		require.NoError(t, err)
		resultBundleEntry, notifications, err := resultHandler(&txResultBundle)

		require.NoError(t, err)
		require.Empty(t, notifications)
		assert.JSONEq(t, string(expectedServiceRequestJson), string(resultBundleEntry.Resource))
		assert.Equal(t, "ServiceRequest/123", *resultBundleEntry.Response.Location)
	})
	t.Run("handles unmanaged FHIR operations - fail for unmanaged operations", func(t *testing.T) {
		// Default behaviour is that we fail when a user tries to perform an unmanaged operation
		tx := coolfhir.Transaction()
		// The unmanaged operation handler reads the resource to return from the result Bundle,
		// from the same index as the resource it added to the transaction Bundle. To test this,
		// we make sure there's 2 other resources in the Bundle.
		tx.Create(fhir.Task{})
		ctrl := gomock.NewController(t)
		fhirClient := mock.NewMockClient(ctrl)
		service := Service{
			fhirClient: fhirClient,
		}
		request := FHIRHandlerRequest{
			ResourcePath: "ServiceRequest",
			HttpMethod:   http.MethodPost,
			Context:      context.Background(),
		}

		resultHandler, err := service.handleUnmanagedOperation(request, tx)
		require.Error(t, err)
		require.Nil(t, resultHandler)
	})
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
	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))

	var capturedHeaders []http.Header
	service.handlerProvider = func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		switch method {
		case http.MethodPost:
			switch resourceType {
			case "CarePlan":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("CarePlan"))
						carePlan := fhir.CarePlan{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(carePlan)
						return result, []any{&carePlan}, nil
					}, nil
				}
			case "Task":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("Task"))
						task := fhir.Task{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(task)
						return result, []any{&task}, nil
					}, nil
				}
			}
		case http.MethodPut:
			switch resourceType {
			case "Task":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					capturedHeaders = append(capturedHeaders, request.HttpHeaders)
					tx.AppendEntry(request.bundleEntry())
					return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
						result := coolfhir.FirstBundleEntry(txResult, coolfhir.EntryIsOfType("Task"))
						task := fhir.Task{
							Id: to.Ptr("123"),
						}
						result.Resource, _ = json.Marshal(task)
						return result, []any{&task}, nil
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
	service.policyAgent = NewMockPolicyMiddleware()
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
		t.Run("POST with specified ID is disallowed", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPOST,
							Url:    "CarePlan/123",
						},
					},
				},
			}
			var resultBundle fhir.Bundle

			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] CarePlanService/CreateBundle failed: bundle.entry[0]: specifying IDs when creating resources isn't allowed")
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
	service := &Service{profile: prof}

	t.Run("ok", func(t *testing.T) {
		err = service.validateLiteralReferences(context.Background(), resource)
		require.NoError(t, err)
	})
	t.Run("http:// is not allowed", func(t *testing.T) {
		resource := deep.AlterCopy(resource, func(s *fhir.Task) {
			s.Focus.Reference = to.Ptr("http://example.com")
		})
		err := service.validateLiteralReferences(context.Background(), resource)
		require.EqualError(t, err, "literal reference is URL with scheme http://, only https:// is allowed (path=focus.reference)")
	})
	t.Run("parent directory traversal isn't allowed", func(t *testing.T) {
		resource := deep.AlterCopy(resource, func(s *fhir.Task) {
			s.Focus.Reference = to.Ptr("https://example.com/fhir/../secret-page")
		})
		err := service.validateLiteralReferences(context.Background(), resource)
		require.EqualError(t, err, "literal reference is URL with parent path segment '..' (path=focus.reference)")
	})
	t.Run("registered base URL", func(t *testing.T) {
		t.Run("path differs", func(t *testing.T) {
			resource := deep.AlterCopy(resource, func(s *fhir.Task) {
				s.Focus.Reference = to.Ptr("https://example.com/alternate/secret-page")
			})
			err = service.validateLiteralReferences(context.Background(), resource)
			require.EqualError(t, err, "literal reference is not a child of a registered FHIR base URL (path=focus.reference)")
		})
		t.Run("path differs, check trailing slash normalization", func(t *testing.T) {
			resource := deep.AlterCopy(resource, func(s *fhir.Task) {
				s.Focus.Reference = to.Ptr("https://example.com/fhirPatient/not-allowed")
			})
			err = service.validateLiteralReferences(context.Background(), resource)
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
	}, profile.Test(), orcaPublicURL, messageBroker, events.NewManager(messageBroker))
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
