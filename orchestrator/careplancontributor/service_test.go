package careplancontributor

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/external"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/messaging"

	"github.com/rs/zerolog/log"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/sse"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/events"
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

func TestService_Proxy_Get_And_Search(t *testing.T) {
	tests := []struct {
		name string
		// Set if it should be anything other than the default (auth.TestPrincipal1)
		principal *auth.Principal
		// Set if it should be anything other than the default (true)
		healthDataViewEndpointEnabled *bool
		expectedStatus                int
		xSCPContext                   string
		readBodyReturnFile            string
		readStatusReturn              int
		// Allows us to test both GET and POST requests
		patientRequestURL   *string
		patientStatusReturn *int
		expectedJSON        string
		expectedError       error
		// Default is GET
		method *string
		// Default is "/cpc/fhir/Patient/1"
		url          *string
		allowCaching bool
	}{
		{
			name:                          "Fails: No healthDataViewEndpointEnabled flag",
			healthDataViewEndpointEnabled: to.Ptr(false),
			expectedStatus:                http.StatusMethodNotAllowed,
		},
		{
			name:           "Fails: No header",
			expectedStatus: http.StatusBadRequest,
			expectedJSON:   `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: X-Scp-Context header must be set"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:           "Fails: header resource is not CarePlan",
			expectedStatus: http.StatusBadRequest,
			xSCPContext:    "SomeResource/invalid",
			expectedJSON:   `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: specified SCP context header does not refer to a CarePlan"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CarePlan in header not found - GET",
			expectedStatus:     http.StatusNotFound,
			readStatusReturn:   http.StatusNotFound,
			readBodyReturnFile: "./testdata/careplan-not-found.json",
			xSCPContext:        "CarePlan/not-exists",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: OperationOutcome, issues: [not-found error] CarePlan not found"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CarePlan in header not found - POST",
			expectedStatus:     http.StatusNotFound,
			readStatusReturn:   http.StatusNotFound,
			readBodyReturnFile: "./testdata/careplan-not-found.json",
			xSCPContext:        "CarePlan/not-exists",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/fhir/Patient/_search failed: OperationOutcome, issues: [not-found error] CarePlan not found"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CareTeam not present in bundle - GET",
			expectedStatus:     http.StatusInternalServerError,
			readBodyReturnFile: "./testdata/careplan-careteam-missing.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: invalid CareTeam reference (must be a reference to a contained resource): CareTeam/cps-careteam-01"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CareTeam not present in bundle - POST",
			expectedStatus:     http.StatusInternalServerError,
			readBodyReturnFile: "./testdata/careplan-careteam-missing.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/fhir/Patient/_search failed: invalid CareTeam reference (must be a reference to a contained resource): CareTeam/cps-careteam-01"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: requester not part of CareTeam - GET",
			principal:          auth.TestPrincipal3,
			expectedStatus:     http.StatusForbidden,
			readBodyReturnFile: "./testdata/careplan-valid.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: requester not part of CareTeam - POST",
			principal:          auth.TestPrincipal3,
			expectedStatus:     http.StatusForbidden,
			readBodyReturnFile: "./testdata/careplan-valid.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/fhir/Patient/_search failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                "Fails: proxied request returns error - GET",
			expectedStatus:      http.StatusNotFound,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientRequestURL:   to.Ptr("GET /fhir/Patient/1"),
			patientStatusReturn: to.Ptr(http.StatusNotFound),
		},
		{
			name:                "Fails: proxied request returns error - POST",
			expectedStatus:      http.StatusNotFound,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientRequestURL:   to.Ptr("GET /fhir/Patient/1"),
			patientStatusReturn: to.Ptr(http.StatusNotFound),
			method:              to.Ptr("POST"),
			url:                 to.Ptr("/cpc/fhir/Patient/_search"),
		},
		{
			name:                "Fails: requester is CareTeam member but Period is expired - GET",
			principal:           auth.TestPrincipal2,
			expectedStatus:      http.StatusForbidden,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientRequestURL:   to.Ptr("GET /fhir/Patient/1"),
			patientStatusReturn: to.Ptr(http.StatusOK),
			expectedJSON:        `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/fhir/Patient/1 failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                "Fails: requester is CareTeam member but Period is expired - POST",
			principal:           auth.TestPrincipal2,
			expectedStatus:      http.StatusForbidden,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientRequestURL:   to.Ptr("GET /fhir/Patient/1"),
			patientStatusReturn: to.Ptr(http.StatusOK),
			method:              to.Ptr("POST"),
			url:                 to.Ptr("/cpc/fhir/Patient/_search"),
			expectedJSON:        `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/fhir/Patient/_search failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                "Success: valid request - GET",
			expectedStatus:      http.StatusOK,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			patientRequestURL:   to.Ptr("/cpc/fhir/Patient/1"),
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientStatusReturn: to.Ptr(http.StatusOK),
		},
		{
			name:                "Success: valid request - GET - Allow caching",
			expectedStatus:      http.StatusOK,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			patientRequestURL:   to.Ptr("/cpc/fhir/Patient/1"),
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientStatusReturn: to.Ptr(http.StatusOK),
			allowCaching:        true,
		},
		{
			name:                "Success: valid request - POST",
			expectedStatus:      http.StatusOK,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			patientRequestURL:   to.Ptr("/cpc/fhir/Patient/_search"),
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientStatusReturn: to.Ptr(http.StatusOK),
			method:              to.Ptr("POST"),
			url:                 to.Ptr("/cpc/fhir/Patient/_search"),
		},
		{
			name:                "Success: valid request - POST - Allow caching",
			expectedStatus:      http.StatusOK,
			readBodyReturnFile:  "./testdata/careplan-valid.json",
			patientRequestURL:   to.Ptr("/cpc/fhir/Patient/_search"),
			readStatusReturn:    http.StatusOK,
			xSCPContext:         "CarePlan/cps-careplan-01",
			patientStatusReturn: to.Ptr(http.StatusOK),
			method:              to.Ptr("POST"),
			url:                 to.Ptr("/cpc/fhir/Patient/_search"),
			allowCaching:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fhirServerMux := http.NewServeMux()
			fhirServer := httptest.NewServer(fhirServerMux)
			fhirServerURL, _ := url.Parse(fhirServer.URL)
			sessionManager, _ := createTestSession()
			messageBroker, err := messaging.New(messaging.Config{}, nil)
			require.NoError(t, err)

			carePlanServiceMux := http.NewServeMux()
			carePlanService := httptest.NewServer(carePlanServiceMux)
			carePlanServiceURL, _ := url.Parse(carePlanService.URL)
			carePlanServiceURL.Path = "/cps"

			healthDataViewEndpointEnabled := true
			if tt.healthDataViewEndpointEnabled != nil {
				healthDataViewEndpointEnabled = *tt.healthDataViewEndpointEnabled
			}

			proxy := coolfhir.NewProxy(
				"MockProxy",
				fhirServerURL,
				"/cpc/cps/fhir",
				orcaPublicURL.JoinPath("/cpc/cps/fhir"),
				http.DefaultTransport,
				tt.allowCaching, false,
			)

			service, _ := New(Config{
				FHIR: coolfhir.ClientConfig{
					BaseURL: fhirServer.URL + "/fhir",
				},
				HealthDataViewEndpointEnabled: healthDataViewEndpointEnabled,
			}, profile.Test(), orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), proxy, carePlanServiceURL, nil)

			// Setup: configure the service to proxy to the backing FHIR server
			frontServerMux := http.NewServeMux()

			if tt.readBodyReturnFile != "" {
				carePlanServiceMux.HandleFunc(fmt.Sprintf("GET /cps/%s", tt.xSCPContext), func(writer http.ResponseWriter, request *http.Request) {
					rawJson, _ := os.ReadFile(tt.readBodyReturnFile)
					writer.WriteHeader(tt.readStatusReturn)
					_, _ = writer.Write(rawJson)
				})
			}
			if tt.patientRequestURL != nil && tt.patientStatusReturn != nil {
				fhirServerMux.HandleFunc(*tt.patientRequestURL, func(writer http.ResponseWriter, request *http.Request) {
					writer.WriteHeader(*tt.patientStatusReturn)
					_ = json.NewEncoder(writer).Encode(fhir.Patient{})
				})
			}

			service.RegisterHandlers(frontServerMux)
			frontServer := httptest.NewServer(frontServerMux)

			if tt.xSCPContext != "" {
				tt.xSCPContext = fmt.Sprintf("%s/%s", carePlanServiceURL, tt.xSCPContext)
			}

			httpClient := frontServer.Client()
			principal := auth.TestPrincipal1
			if tt.principal != nil {
				principal = tt.principal
			}
			httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, principal, tt.xSCPContext)

			method := "GET"
			if tt.method != nil {
				method = *tt.method
			}
			reqURL := "/cpc/fhir/Patient/1"
			if tt.url != nil {
				reqURL = *tt.url
			}
			log.Info().Msgf("Requesting %s %s", method, frontServer.URL+reqURL)
			log.Info().Msgf("FHIR Server URL: %s", fhirServer.URL)
			log.Info().Msgf("CarePlan Service URL: %s", carePlanService.URL)

			httpRequest, _ := http.NewRequest(method, frontServer.URL+reqURL, nil)
			httpResponse, err := httpClient.Do(httpRequest)
			require.Equal(t, err, tt.expectedError)
			require.Equal(t, tt.expectedStatus, httpResponse.StatusCode)

			if tt.expectedJSON != "" {
				body, _ := io.ReadAll(httpResponse.Body)
				require.JSONEq(t, tt.expectedJSON, string(body))
			}
			if tt.readBodyReturnFile != "" {
				if tt.expectedStatus == http.StatusOK {
					if tt.allowCaching {
						assert.Equal(t, "must-understand, private", httpResponse.Header.Get("Cache-Control"))
					} else {
						assert.Equal(t, "no-store", httpResponse.Header.Get("Cache-Control"))
					}
				}
			}
		})
	}
}

// Invalid test cases are simpler, can be tested with http endpoint mocking
func TestService_HandleNotification_Invalid(t *testing.T) {
	prof := profile.Test()
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)

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
	}, profile.Test(), orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), &httputil.ReverseProxy{}, must.ParseURL(fhirServer.URL), nil)

	frontServerMux := http.NewServeMux()
	frontServer := httptest.NewServer(frontServerMux)
	service.RegisterHandlers(frontServerMux)
	ctx := context.Background()
	httpClient, _ := prof.HttpClient(ctx, auth.TestPrincipal1.Organization.Identifier[0])

	t.Run("invalid notification - wrong data type", func(t *testing.T) {
		notification := fhir.Task{Id: to.Ptr("1")}
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
	t.Run("valid notification (no trailing slash)", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Patient/1"), Type: to.Ptr("Patient")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification (with trailing slash)", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Patient/1"), Type: to.Ptr("Patient")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir/", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not found", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/999"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not SCP", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/1"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - invalid task missing focus", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/2"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

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
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader("invalid"))
		httpResponse, err := httpClient.Do(httpRequest)

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
	httpClient, _ := prof.HttpClient(nil, auth.TestPrincipal2.Organization.Identifier[0])
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)

	service, _ := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, profile.TestProfile{
		Principal: auth.TestPrincipal2,
	}, orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), &httputil.ReverseProxy{}, must.ParseURL(fhirServer.URL), nil)
	service.workflows = taskengine.DefaultTestWorkflowProvider()

	var capturedFhirBaseUrl string
	t.Cleanup(func() {
		fhirClientFactory = createFHIRClient
	})
	fhirClientFactory = func(fhirBaseURL *url.URL, _ *http.Client) fhirclient.Client {
		capturedFhirBaseUrl = fhirBaseURL.String()
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

	httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/fhir", strings.NewReader(string(notificationJSON)))
	httpResponse, err := httpClient.Do(httpRequest)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.String(), capturedFhirBaseUrl)
}

func TestService_Proxy_ProxyToEHR_WithLogout(t *testing.T) {
	// Test that the service registers the EHR FHIR proxy URL that proxies to the backing FHIR server of the EHR
	// Setup: configure backing EHR FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient/1", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
		_ = json.NewEncoder(writer).Encode(fhir.Patient{})
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
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)
	sessionManager, sessionID := createTestSession()

	service, err := New(Config{}, profile.Test(), orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), &httputil.ReverseProxy{}, must.ParseURL(fhirServer.URL), nil)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/ehr/fhir/Patient/1", nil)
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

	httpRequest, _ = http.NewRequest("GET", frontServer.URL+"/cpc/ehr/fhir/Patient/1", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err = frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, httpResponse.StatusCode)
}

func TestService_HandleSearchEndpoints(t *testing.T) {
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	sessionManager, _ := createTestSession()
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)

	service, err := New(Config{
		AppLaunch: applaunch.Config{
			External: map[string]external.Config{
				"app1": {
					Name: "App 1",
					URL:  "https://example.com/app1",
				},
				"app2": {
					Name: "App 2",
					URL:  "https://example.com/app2",
				},
			},
		},
	}, profile.Test(), orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), &httputil.ReverseProxy{}, nil, nil)
	require.NoError(t, err)

	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)
	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("ok", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Endpoint", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		expectedData, err := os.ReadFile("./testdata/endpoints.json")
		require.NoError(t, err)
		require.JSONEq(t, string(expectedData), string(responseData))
	})
	t.Run("search parameters not allowed", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/fhir/Endpoint?foo=bar", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
}

func TestService_handleGetContext(t *testing.T) {
	t.Run("everything present", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()
		sessionData := session.Data{
			TaskIdentifier: to.Ptr("task-identifier-123"),
		}
		sessionData.Set("Practitioner/the-doctor", nil)
		sessionData.Set("PractitionerRole/the-doctor-role", nil)
		sessionData.Set("ServiceRequest/1", nil)
		sessionData.Set("Patient/1", nil)
		sessionData.Set("Task/1", nil)
		Service{}.handleGetContext(httpResponse, nil, &sessionData)
		assert.Equal(t, http.StatusOK, httpResponse.Code)
		assert.JSONEq(t, `{
		"practitioner": "Practitioner/the-doctor",
		"practitionerRole": "PractitionerRole/the-doctor-role",
		"serviceRequest": "ServiceRequest/1",
		"patient": "Patient/1",
		"task": "Task/1",
		"taskIdentifier": "task-identifier-123"
	}`, httpResponse.Body.String())
	})
	t.Run("no PractitionerRole", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()
		sessionData := session.Data{
			TaskIdentifier: to.Ptr("task-identifier-123"),
		}
		sessionData.Set("Practitioner/the-doctor", nil)
		sessionData.Set("ServiceRequest/1", nil)
		sessionData.Set("Patient/1", nil)
		sessionData.Set("Task/1", nil)
		Service{}.handleGetContext(httpResponse, nil, &sessionData)
		assert.Equal(t, http.StatusOK, httpResponse.Code)
		assert.JSONEq(t, `{
		"practitioner": "Practitioner/the-doctor",
		"practitionerRole": "",
		"serviceRequest": "ServiceRequest/1",
		"patient": "Patient/1",
		"task": "Task/1",
		"taskIdentifier": "task-identifier-123"
	}`, httpResponse.Body.String())
	})
}

func TestService_proxyToAllCareTeamMembers(t *testing.T) {
	// TODO: Non-happy tests:
	//       - CarePlan not found
	//       - Non-active members aren't queried
	//       - etc...
	//       - duplicate endpoints; aren't queried twice
	t.Run("ok", func(t *testing.T) {
		remoteContributorFHIRAPIMux := http.NewServeMux()
		// CPC endpoints
		remoteContributorFHIRAPIMux.HandleFunc("POST /cpc/fhir/Patient/_search", func(writer http.ResponseWriter, request *http.Request) {
			results := coolfhir.SearchSet().Append(fhir.Patient{
				Id: to.Ptr("1"),
			}, nil, nil)
			coolfhir.SendResponse(writer, http.StatusOK, results)
		})
		// CPS endpoints
		carePlanData, err := os.ReadFile("./testdata/careplan-valid.json")
		require.NoError(t, err)
		var carePlan fhir.CarePlan
		require.NoError(t, json.Unmarshal(carePlanData, &carePlan))

		remoteContributorFHIRAPIMux.HandleFunc("GET /cps/fhir/CarePlan/1", func(writer http.ResponseWriter, request *http.Request) {
			coolfhir.SendResponse(writer, http.StatusOK, carePlan)
		})

		remoteContributorFHIRAPI := httptest.NewServer(remoteContributorFHIRAPIMux)
		cpcBaseURL := remoteContributorFHIRAPI.URL + "/cpc/fhir"
		cpsBaseURL := remoteContributorFHIRAPI.URL + "/cps/fhir"
		scpContext := cpsBaseURL + "/CarePlan/1"

		publicURL, _ := url.Parse("https://example.com")
		service := &Service{
			orcaPublicURL: publicURL,
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal2,
				CSD: profile.TestCsdDirectory{
					Endpoints: map[string]map[string]string{
						"http://fhir.nl/fhir/NamingSystem/ura|2": {
							"fhirBaseURL": "http://example.com",
						},
						"http://fhir.nl/fhir/NamingSystem/ura|1": {
							"fhirBaseURL": cpcBaseURL,
						},
					},
				},
			},
		}
		service.createFHIRClientForURL = service.defaultCreateFHIRClientForURL
		expectedBody := url.Values{}

		httpRequest := httptest.NewRequest("POST", publicURL.JoinPath("/cpc/aggregate/fhir/Patient/_search").String(), strings.NewReader(expectedBody.Encode()))
		httpRequest.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		httpRequest.Header.Add("X-SCP-Context", scpContext)
		httpResponse := httptest.NewRecorder()

		err = service.proxyToAllCareTeamMembers(httpResponse, httpRequest)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.Code)
		var actualBundle fhir.Bundle
		require.NoError(t, json.Unmarshal(httpResponse.Body.Bytes(), &actualBundle))
		require.Len(t, actualBundle.Entry, 1)
	})
}

func TestService_HandleSubscribeToTask(t *testing.T) {

	validToken := "http://fhir.nl/fhir/NamingSystem/task-workflow-identifier|12345"

	// Create a dummy local CarePlanService URL.
	localCPSUrl, err := url.Parse("http://dummy-cps")
	require.NoError(t, err)

	tests := []struct {
		name            string
		sessionData     session.Data
		client          mock.MockClient
		setLocalCPS     bool
		expectedStatus  int
		expectedContent string
	}{
		{
			name:            "No taskIdentifier in session",
			sessionData:     session.Data{},
			setLocalCPS:     true,
			expectedStatus:  http.StatusBadRequest,
			expectedContent: "No taskIdentifier found in session",
		},
		{
			name: "Invalid taskIdentifier in session",
			sessionData: session.Data{
				TaskIdentifier: to.Ptr("invalid-token"),
			},
			setLocalCPS:     true,
			expectedStatus:  http.StatusBadRequest,
			expectedContent: "Invalid taskIdentifier in session",
		},
		{
			name: "No local CarePlanService configured",
			sessionData: session.Data{
				TaskIdentifier: &validToken,
			},
			setLocalCPS:     false,
			expectedStatus:  http.StatusBadRequest,
			expectedContent: "No local CarePlanService configured",
		},
		{
			name: "Task identifier does not match session",
			sessionData: session.Data{
				TaskIdentifier: to.Ptr("https://some.other.domain/fhir/NamingSystem/task-workflow-identifier|12345"),
			},
			setLocalCPS:     true,
			expectedStatus:  http.StatusBadRequest,
			expectedContent: "Task identifier does not match the taskIdentifier in the session",
		},
		{
			name: "Success subscription",
			sessionData: session.Data{
				TaskIdentifier: to.Ptr(validToken),
			},
			setLocalCPS:     true,
			expectedStatus:  http.StatusOK,
			expectedContent: "",
		},
	}

	ctx := context.Background()
	rawJson, _ := os.ReadFile("./testdata/task-3.json")

	var taskData map[string]interface{}
	err = json.Unmarshal(rawJson, &taskData)
	require.NoError(t, err)
	mockFhirClient := &test.StubFHIRClient{
		Resources: []any{taskData},
	}

	sseService := sse.New()
	sseService.ServeHTTP = func(topic string, writer http.ResponseWriter, request *http.Request) {
		log.Ctx(ctx).Info().Msgf("Unit-Test: Transform request to SSE stream for topic: %s", topic)
	}

	svc := &Service{sseService: sseService, profile: profile.TestProfile{Principal: auth.TestPrincipal1}}

	svc.createFHIRClientForURL = func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error) {
		return mockFhirClient, nil, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &tt.sessionData

			req := httptest.NewRequest("GET", "/cpc/subscribe/fhir/Task/3", nil)
			req.SetPathValue("id", "3")

			resp := httptest.NewRecorder()

			if tt.setLocalCPS {
				svc.localCarePlanServiceUrl = localCPSUrl
			} else {
				svc.localCarePlanServiceUrl = nil
			}

			// Call method under test.
			svc.handleSubscribeToTask(resp, req, session)

			res := resp.Result()
			defer res.Body.Close()
			bodyBytes, _ := io.ReadAll(res.Body)
			bodyStr := string(bodyBytes)
			assert.Equal(t, tt.expectedStatus, res.StatusCode, "Unexpected status code")
			assert.Contains(t, bodyStr, tt.expectedContent, "Unexpected error message or response text, got: "+bodyStr)
		})
	}
}

func TestService_ExternalFHIRProxy(t *testing.T) {
	t.Log("This tests the external FHIR proxy functionality (/cpc/external/fhir, used by the EHR and ORCA Frontend to query remote SCP-nodes' FHIR APIs.")

	remoteFHIRAPIMux := http.NewServeMux()
	remoteFHIRAPIMux.HandleFunc("GET /fhir/Task/1", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{Id: to.Ptr("1")})
	})
	remoteSCPNode := httptest.NewServer(remoteFHIRAPIMux)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /fhir/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{Id: to.Ptr("2")})
	})

	httpServer := httptest.NewServer(mux)
	service := &Service{
		profile: profile.TestProfile{
			Principal: auth.TestPrincipal1,
			CSD: profile.TestCsdDirectory{
				Endpoints: map[string]map[string]string{
					"http://fhir.nl/fhir/NamingSystem/ura|2": {
						"fhirBaseURL": remoteSCPNode.URL + "/fhir",
					},
				},
			},
		},
		httpHandler:             mux,
		localCarePlanServiceUrl: must.ParseURL(httpServer.URL + "/fhir"),
		orcaPublicURL:           must.ParseURL(httpServer.URL),
		config:                  Config{StaticBearerToken: "secret"},
	}
	service.createFHIRClientForURL = service.defaultCreateFHIRClientForURL
	service.RegisterHandlers(mux)

	t.Run("X-Scp-Entity-Identifier", func(t *testing.T) {
		httpRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/cpc/external/fhir/Task/1", nil)
		httpRequest.Header.Set("Authorization", "Bearer secret")
		httpRequest.Header.Set("X-Scp-Entity-Identifier", "http://fhir.nl/fhir/NamingSystem/ura|2")
		httpResponse, err := httpServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err)
		assert.NotEmpty(t, responseData)

		t.Run("assert meta source is set", func(t *testing.T) {
			var bundle fhir.Bundle
			err = json.Unmarshal(responseData, &bundle)
			require.NoError(t, err)
			assert.Equal(t, remoteSCPNode.URL+"/fhir/Task/1", *bundle.Meta.Source)
		})
	})
	t.Run("X-Scp-Fhir-Url", func(t *testing.T) {
		t.Run("URL, external", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/cpc/external/fhir/Task/1", nil)
			httpRequest.Header.Set("Authorization", "Bearer secret")
			httpRequest.Header.Set("X-Scp-Fhir-Url", remoteSCPNode.URL+"/fhir")
			httpResponse, err := httpServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			responseData, err := io.ReadAll(httpResponse.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, responseData)
		})
		t.Run("URL, local", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/cpc/external/fhir/Task/2", nil)
			httpRequest.Header.Set("Authorization", "Bearer secret")
			httpRequest.Header.Set("X-Scp-Fhir-Url", service.localCarePlanServiceUrl.String())
			httpResponse, err := httpServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			responseData, err := io.ReadAll(httpResponse.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, responseData)
		})
		t.Run("local-cps", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/cpc/external/fhir/Task/2", nil)
			httpRequest.Header.Set("Authorization", "Bearer secret")
			httpRequest.Header.Set("X-Scp-Fhir-Url", "local-cps")
			httpResponse, err := httpServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			responseData, err := io.ReadAll(httpResponse.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, responseData)
		})
	})
	t.Run("can't determine remote node", func(t *testing.T) {
		httpRequest, _ := http.NewRequest(http.MethodPost, httpServer.URL+"/cpc/external/fhir/Task/_search", nil)
		httpRequest.Header.Set("Authorization", "Bearer secret")
		httpResponse, err := httpServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
		responseData, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err)
		assert.Contains(t, string(responseData), "can't determine the external SCP-node to query from the HTTP request headers")
	})
}

func createTestSession() (*user.SessionManager[session.Data], string) {
	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	sessionHttpResponse := httptest.NewRecorder()
	sessionManager.Create(sessionHttpResponse, session.Data{
		FHIRLauncher: "test",
	})
	// extract session ID; s_id=<something>;
	cookieValue := sessionHttpResponse.Header().Get("Set-Cookie")
	cookieValue = strings.Split(cookieValue, ";")[0]
	cookieValue = strings.Split(cookieValue, "=")[1]
	return sessionManager, cookieValue
}
