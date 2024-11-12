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
	fhirServer := httptest.NewServer(fhirServerMux)
	// Setup: create the service
	service, err := New(Config{
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

	t.Run("GET Patient - not allowed", func(t *testing.T) {
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Patient")
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, httpResponse.StatusCode)
	})
	t.Run("POST/Create Questionnaire - not allowed", func(t *testing.T) {
		httpResponse, err := httpClient.Post(frontServer.URL+"/cps/Questionnaire", "application/fhir+json", nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, httpResponse.StatusCode)
	})
	t.Run("PUT/Update Questionnaire - allowed", func(t *testing.T) {
		fhirServerMux.HandleFunc("POST /", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			resp := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Response: &fhir.BundleEntryResponse{
							Location: to.Ptr("Questionnaire/1"),
							Status:   "200 OK",
						},
					},
				},
			}
			_ = json.NewEncoder(writer).Encode(resp)
		})
		fhirServerMux.HandleFunc("GET /fhir/Questionnaire/1", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(writer).Encode(fhir.Questionnaire{
				Id: to.Ptr("1"),
			})
		})

		request, err := http.NewRequest(http.MethodPut, frontServer.URL+"/cps/Questionnaire/1", nil)
		require.NoError(t, err)
		response, err := httpClient.Do(request)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
	})
	t.Run("GET Questionnaire - allowed", func(t *testing.T) {
		fhirServerMux.HandleFunc("GET /fhir/Questionnaire", func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusOK)
		})
		httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Questionnaire")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("DELETE Questionnaire - not supported", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodDelete, frontServer.URL+"/cps/Questionnaire/1", nil)
		require.NoError(t, err)
		httpResponse, err := httpClient.Do(request)
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, httpResponse.StatusCode)
	})

	// Unauthorised Get
	t.Run("GET Questionnaire - not authorised, not allowed", func(t *testing.T) {
		httpClient.Transport = http.DefaultTransport
		response, err := httpClient.Get(frontServer.URL + "/cps/Questionnaire")
		require.NoError(t, err)
		require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	})
}

func TestService_Proxy_AllowUnmanagedOperations(t *testing.T) {
	// Test that the service registers the /cps URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
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

	httpResponse, err := httpClient.Get(frontServer.URL + "/cps/Patient")
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
		service := Service{
			fhirClient:                   fhirClient,
			allowUnmanagedFHIROperations: true,
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
		if strings.Contains(string(capturedRequestBody), "THIS-MUST-FAIL") {
			coolfhir.WriteOperationOutcomeFromError(coolfhir.BadRequestError("this must fail"), "Failed", writer)
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

	service.handlerProvider = func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
		switch method {
		case http.MethodPost:
			switch resourceType {
			case "CarePlan":
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
					tx.Append(request.bundleEntry())
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
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
					tx.Append(request.bundleEntry())
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
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
					tx.Append(request.bundleEntry())
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
				return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
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
		t.Run("commit fails", func(t *testing.T) {
			requestBundle := fhir.Bundle{
				Type: fhir.BundleTypeTransaction,
				Entry: []fhir.BundleEntry{
					{
						Request: &fhir.BundleEntryRequest{
							Method: fhir.HTTPVerbPUT,
							Url:    "Task",
						},
						Resource: json.RawMessage(`{"intent": "THIS-MUST-FAIL"}`),
					},
				},
			}
			var resultBundle fhir.Bundle

			hdrs := new(fhirclient.Headers)
			err = fhirClient.Create(requestBundle, &resultBundle, fhirclient.AtPath("/"), fhirclient.ResponseHeaders(hdrs))

			require.EqualError(t, err, "OperationOutcome, issues: [processing error] Bundle failed: upstream FHIR server error")
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
