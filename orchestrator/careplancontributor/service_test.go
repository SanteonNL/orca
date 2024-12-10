package careplancontributor

import (
	"encoding/json"
	"fmt"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"

	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")

// TestService_ExternalCPCProxyToEHR tests the proxies that external CPCs use to request data from the local EHR's FHIR API.
func TestService_ExternalCPCProxyToEHR(t *testing.T) {
	t.Skip("Fix test once the INT-487 is picked up") //TODO: Fix test once the INT-487 is picked up

	fhirServerMux := http.NewServeMux()
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(writer).Encode(fhir.Bundle{})
	})

	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	carePlanServiceMux := http.NewServeMux()
	capturedCarePlanSearchRequestUrl := ""
	carePlanServiceMux.HandleFunc("GET /cps/CarePlan", func(writer http.ResponseWriter, request *http.Request) {
		capturedCarePlanSearchRequestUrl = request.URL.String()
		if strings.Contains(request.URL.RawQuery, "not-exists") {
			rawJson, _ := os.ReadFile("./testdata/careplan-bundle-not-found.json")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(rawJson)
			return
		}
		if strings.Contains(request.URL.RawQuery, "careteam-missing") {
			rawJson, _ := os.ReadFile("./testdata/careplan-bundle-careteam-missing.json")
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(rawJson)
			return
		}
		rawJson, _ := os.ReadFile("./testdata/careplan-bundle-valid.json")
		var data fhir.Bundle
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/cps"

	service, _ := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
		CarePlanService: CarePlanServiceConfig{
			URL: carePlanServiceURL.String(),
		},
		HealthDataViewEndpointEnabled: true,
	}, profile.TestProfile{}, orcaPublicURL, sessionManager, &httputil.ReverseProxy{})
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := &http.Client{}
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	carePlanUrl := fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL)

	t.Run("ok", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
		httpRequest.Header.Set("X-SCP-Context", carePlanUrl)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedCarePlanSearchRequestUrl)
		t.Run("caching is allowed", func(t *testing.T) {
			assert.Equal(t, "must-understand, private", httpResponse.Header.Get("Cache-Control"))
		})
	})
	t.Run("upstream FHIR server returns 404 for resource", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient/100", nil)
		httpRequest.Header.Set("X-SCP-Context", carePlanUrl)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, httpResponse.StatusCode)
	})
	t.Run("endpoint not enabled", func(t *testing.T) {
		service.healthdataviewEndpointEnabled = false
		t.Cleanup(func() {
			service.healthdataviewEndpointEnabled = true
		})
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusMethodNotAllowed, httpResponse.StatusCode)
	})
	t.Run("x-scp-context header", func(t *testing.T) {
		t.Run("header not present", func(t *testing.T) {
			httpResponse, err := httpClient.Get(frontServer.URL + "/cpc/fhir/Patient")
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			body, _ := io.ReadAll(httpResponse.Body)
			require.Contains(t, string(body), "X-Scp-Context header must be set")
		})
		t.Run("multiple values", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
			httpRequest.Header["X-SCP-Context"] = []string{
				fmt.Sprintf("%s/CarePlan/1", carePlanServiceURL),
				fmt.Sprintf("%s/CarePlan/2", carePlanServiceURL),
			}
			httpResponse, err := httpClient.Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			body, _ := io.ReadAll(httpResponse.Body)
			require.Contains(t, string(body), "X-Scp-Context header can't contain multiple values")
		})
		t.Run("header does not refer to a CarePlan", func(t *testing.T) {
			httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
			httpRequest.Header["X-SCP-Context"] = []string{fmt.Sprintf("%s/SomeResource/invalid", carePlanServiceURL)}
			httpResponse, err := httpClient.Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
			body, _ := io.ReadAll(httpResponse.Body)
			require.JSONEq(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /cpc/fhir/Patient failed: specified SCP context header does not refer to a CarePlan"}],"resourceType":"OperationOutcome"}`, string(body))
		})
	})
	t.Run("CarePlan not found", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
		httpRequest.Header["X-SCP-Context"] = []string{fmt.Sprintf("%s/CarePlan/not-exists", carePlanServiceURL)}
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, httpResponse.StatusCode)
		body, _ := io.ReadAll(httpResponse.Body)
		require.Contains(t, string(body), "CarePlan not found")
	})
	t.Run("CareTeam not present", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
		httpRequest.Header["X-SCP-Context"] = []string{fmt.Sprintf("%s/CarePlan/careteam-missing", carePlanServiceURL)}
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, httpResponse.StatusCode)
		body, _ := io.ReadAll(httpResponse.Body)
		require.Contains(t, string(body), "CareTeam not found in bundle")
	})
	t.Run("no access", func(t *testing.T) {
		t.Run("requester not in CareTeam", func(t *testing.T) {
			ts := auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal3, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL))

			httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
			httpResponse, err := ts.RoundTrip(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, httpResponse.StatusCode)
			body, _ := io.ReadAll(httpResponse.Body)
			require.JSONEq(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /cpc/fhir/Patient failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`, string(body))
			require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedCarePlanSearchRequestUrl)
		})
		t.Run("active period expired", func(t *testing.T) {
			ts := auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal2, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL.String()))

			httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Patient", nil)
			httpResponse, err := ts.RoundTrip(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, httpResponse.StatusCode)
			body, _ := io.ReadAll(httpResponse.Body)
			require.JSONEq(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /cpc/fhir/Patient failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`, string(body))
			require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedCarePlanSearchRequestUrl)
		})
	})
}

// Invalid test cases are simpler, can be tested with http endpoint mocking
func TestService_HandleNotification_Invalid(t *testing.T) {
	prof := profile.TestProfile{
		Principal: auth.TestPrincipal1,
	}
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/Task/999", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	})
	carePlanServiceMux.HandleFunc("GET /cps/Task/1", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-1.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	carePlanServiceMux.HandleFunc("GET /cps/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-2.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	var capturedTaskUpdate fhir.Task
	carePlanServiceMux.HandleFunc("PUT /cps/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(rawJson, &capturedTaskUpdate)
		writer.WriteHeader(http.StatusOK)
	})
	carePlanServiceMux.HandleFunc("GET /cps/Task/3", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-3.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/cps"

	service, _ := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
		CarePlanService: CarePlanServiceConfig{
			URL: carePlanServiceURL.String(),
		},
	}, profile.TestProfile{
		Principal: auth.TestPrincipal1,
	}, orcaPublicURL, sessionManager, &httputil.ReverseProxy{})

	frontServerMux := http.NewServeMux()
	frontServer := httptest.NewServer(frontServerMux)
	service.RegisterHandlers(frontServerMux)

	t.Run("invalid notification - wrong data type", func(t *testing.T) {
		notification := fhir.Task{Id: to.Ptr("1")}
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
	t.Run("valid notification - unsupported type", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Patient/1"), Type: to.Ptr("Patient")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not found", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/999"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not SCP", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/1"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - invalid task missing focus", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/2"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		// processed OK: Task was invalid, but it was rejected. So the notification itself succeeded
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		// check rejection
		assert.NotNil(t, capturedTaskUpdate.Id)
		assert.Equal(t, "2", *capturedTaskUpdate.Id)
		assert.Equal(t, fhir.TaskStatusRejected, capturedTaskUpdate.Status)
		assert.Equal(t, "Task is not valid: validation errors: Task.Focus is required but not provided", *capturedTaskUpdate.StatusReason.Text)

	})
	t.Run("invalid notification", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader("invalid"))
		httpResponse, err := prof.HttpClient().Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
}

// Valid test case is more complex, use client mocking to simulate data return
func TestService_HandleNotification_Valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	prof := profile.TestProfile{
		Principal: auth.TestPrincipal2,
	}
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	service, _ := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
		CarePlanService: CarePlanServiceConfig{
			URL: fhirServerURL.String(),
		},
	}, profile.TestProfile{
		Principal: auth.TestPrincipal2,
	}, orcaPublicURL, sessionManager, &httputil.ReverseProxy{})
	service.workflows = taskengine.DefaultTestWorkflowProvider()

	var capturedFhirBaseUrl string
	service.cpsClientFactory = func(baseUrl *url.URL) fhirclient.Client {
		capturedFhirBaseUrl = baseUrl.String()
		return mockFHIRClient
	}

	frontServerMux := http.NewServeMux()
	frontServer := httptest.NewServer(frontServerMux)
	service.RegisterHandlers(frontServerMux)

	notification := coolfhir.CreateSubscriptionNotification(fhirServerURL,
		time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/3"), Type: to.Ptr("Task")})
	notificationJSON, _ := json.Marshal(notification)

	mockFHIRClient.EXPECT().Read(fhirServerURL.String()+"/Task/3", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result *fhir.Task, option ...fhirclient.Option) error {
		rawJson, _ := os.ReadFile("./testdata/task-3.json")
		return json.Unmarshal(rawJson, &result)
	})
	mockFHIRClient.EXPECT().Read("ServiceRequest/1", gomock.Any(), gomock.Any()).
		DoAndReturn(func(path string, result *fhir.ServiceRequest, option ...fhirclient.Option) error {
			rawJson, _ := os.ReadFile("./testdata/servicerequest-1.json")
			return json.Unmarshal(rawJson, &result)
		})

	mockFHIRClient.EXPECT().
		Create(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(bundle fhir.Bundle, result interface{}, options ...fhirclient.Option) error {
			mockResponse := map[string]interface{}{
				"id":           uuid.NewString(),
				"resourceType": "Bundle",
				"type":         "transaction-response",
				"entry": []interface{}{
					map[string]interface{}{
						"response": map[string]interface{}{
							"status":   "201 Created",
							"location": "Task/" + uuid.NewString(),
						},
					},
				},
			}
			bytes, _ := json.Marshal(mockResponse)
			json.Unmarshal(bytes, &result)
			return nil
		})

	httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/notify", strings.NewReader(string(notificationJSON)))
	httpResponse, err := prof.HttpClient().Do(httpRequest)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.String(), capturedFhirBaseUrl)
}

func TestService_ProxyToEHR(t *testing.T) {
	// Test that the service registers the EHR FHIR proxy URL that proxies to the backing FHIR server of the EHR
	// Setup: configure backing EHR FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		_ = json.NewEncoder(writer).Encode(fhir.Bundle{})
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	// Setup: create the service

	clients.Factories["test"] = func(properties map[string]string) clients.ClientProperties {
		return clients.ClientProperties{
			Client:  fhirServer.Client().Transport,
			BaseURL: fhirServerURL,
		}
	}
	sessionManager, sessionID := createTestSession()

	service, err := New(Config{}, profile.TestProfile{}, orcaPublicURL, sessionManager, &httputil.ReverseProxy{})
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/ehr/fhir/Patient", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)

	t.Run("caching is not allowed", func(t *testing.T) {
		assert.Equal(t, "no-store", httpResponse.Header.Get("Cache-Control"))
	})

	// Logout and attempt to get the patient again
	httpRequest, _ = http.NewRequest("POST", frontServer.URL+"/logout", nil)

	// Trying to logout without a session cookie should return an error
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, httpResponse.StatusCode)

	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

	httpRequest, _ = http.NewRequest("GET", frontServer.URL+"/cpc/ehr/fhir/Patient", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, httpResponse.StatusCode)
}

func TestService_ProxyToCPS(t *testing.T) {
	// Test that the service registers the CarePlanService FHIR proxy URL that proxies to the CarePlanService
	// Setup: configure CarePlanService to which the service proxies
	carePlanServiceMux := http.NewServeMux()
	capturedHost := ""
	var capturedQueryParams url.Values
	carePlanServiceMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		capturedQueryParams = request.URL.Query()
		_ = json.NewEncoder(writer).Encode(fhir.Bundle{})
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/fhir"

	sessionManager, sessionID := createTestSession()

	service, err := New(Config{
		CarePlanService: CarePlanServiceConfig{
			URL: carePlanServiceURL.String(),
		},
	}, profile.TestProfile{}, orcaPublicURL, sessionManager, &httputil.ReverseProxy{})
	require.NoError(t, err)
	// Setup: configure the service to proxy to the upstream CarePlanService
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/cps/fhir/Patient?_search=foo:bar", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, carePlanServiceURL.Host, capturedHost)
	require.Equal(t, "foo:bar", capturedQueryParams.Get("_search"))

	t.Run("caching is not allowed", func(t *testing.T) {
		assert.Equal(t, "no-store", httpResponse.Header.Get("Cache-Control"))
	})

	// Logout and attempt to get the patient again
	httpRequest, _ = http.NewRequest("POST", frontServer.URL+"/logout", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

	httpRequest, _ = http.NewRequest("GET", frontServer.URL+"/cpc/cps/fhir/Patient?_search=foo:bar", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, httpResponse.StatusCode)
}

func TestService_handleGetContext(t *testing.T) {
	httpResponse := httptest.NewRecorder()
	Service{}.handleGetContext(httpResponse, nil, &user.SessionData{
		StringValues: map[string]string{
			"test":           "value",
			"practitioner":   "the-doctor",
			"serviceRequest": "ServiceRequest/1",
			"patient":        "Patient/1",
		},
	})
	assert.Equal(t, http.StatusOK, httpResponse.Code)
	assert.JSONEq(t, `{
		"practitioner": "the-doctor",
		"serviceRequest": "ServiceRequest/1",	
		"patient": "Patient/1"
	}`, httpResponse.Body.String())
}

func createTestSession() (*user.SessionManager, string) {
	sessionManager := user.NewSessionManager(time.Minute)
	sessionHttpResponse := httptest.NewRecorder()
	sessionManager.Create(sessionHttpResponse, user.SessionData{
		FHIRLauncher: "test",
	})
	// extract session ID; sid=<something>;
	cookieValue := sessionHttpResponse.Header().Get("Set-Cookie")
	cookieValue = strings.Split(cookieValue, ";")[0]
	cookieValue = strings.Split(cookieValue, "=")[1]
	return sessionManager, cookieValue
}
