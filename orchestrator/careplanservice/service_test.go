package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")

func TestService_Proxy(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	var capturedQuery url.Values
	fhirServerMux.HandleFunc("GET /fhir/Success", func(writer http.ResponseWriter, request *http.Request) {
		capturedQuery = request.URL.Query()
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{
			Intent: "order",
		})
	})
	fhirServerMux.HandleFunc("GET /fhir/Fail", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.WriteOperationOutcomeFromError(coolfhir.BadRequest("Fail"), "oops", writer)
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	service, err := New(Config{
		AllowUnmanagedFHIROperations: true,
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.TestProfile{}, orcaPublicURL)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("ok", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		require.JSONEq(t, `{"resourceType":"Task", "intent":"order", "status":"draft"}`, string(responseData))
	})
	t.Run("it proxies query parameters", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Success?_identifier=foo|bar")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		assert.Equal(t, "foo|bar", capturedQuery.Get("_identifier"))
	})
	t.Run("upstream FHIR server returns FHIR error with operation outcome", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Fail")
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
		service, err := New(Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: fhirServer.URL + "/fhir",
			},
		}, profile.TestProfile{}, orcaPublicURL)
		require.NoError(t, err)
		frontServerMux := http.NewServeMux()
		service.RegisterHandlers(frontServerMux)
		frontServer := httptest.NewServer(frontServerMux)

		httpClient := frontServer.Client()
		httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Anything")
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
	fhirServerMux.HandleFunc("GET /fhir/SomeResource", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	// Setup: create the service
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
		AllowUnmanagedFHIROperations: true,
	}, profile.TestProfile{}, orcaPublicURL)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	httpResponse, err := httpClient.Get(frontServer.URL + "/cps/SomeResource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)
}

type OperationOutcomeWithResourceType struct {
	fhir.OperationOutcome
	ResourceType *string `bson:"resourceType" json:"resourceType"`
}

// TestService_ErrorHandling asserts invalid requests return OperationOutcome
func TestService_ErrorHandling(t *testing.T) {
	// Setup: configure the service
	service, err := New(
		Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: "http://localhost",
			},
		},
		profile.TestProfile{},
		orcaPublicURL)
	require.NoError(t, err)
	serverMux := http.NewServeMux()
	service.RegisterHandlers(serverMux)
	server := httptest.NewServer(serverMux)

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
		}
		ctrl := gomock.NewController(t)
		fhirClient := mock.NewMockClient(ctrl)
		fhirClient.EXPECT().Read("ServiceRequest/123", gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ string, resultResource interface{}, opts ...fhirclient.Option) error {
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
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	service, err := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.TestProfile{}, orcaPublicURL)

	service.handlerProvider = func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		switch method {
		case http.MethodPost:
			switch resourceType {
			case "CarePlan":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
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
			// Create a bundle with 2 Tasks
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

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] Bundle failed: OperationOutcome, issues: [processing error] Transaction failed on 'PUT' for the requested url '/ServiceRequest'.; [invalid error] Found result with Id '4eaffb23-02ab-452c-8ef1-b4c9be7b2425', which did not match the provided Id 'e669F4l0Bk3NJpQzoTzVE0opsQR2iWXR41M6FXkeguZo3'.")
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

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] Bundle failed: OperationOutcome, issues: [processing error] upstream FHIR server error")
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
