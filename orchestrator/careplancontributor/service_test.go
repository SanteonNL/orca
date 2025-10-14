package careplancontributor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/external"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/go-test/deep"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"

	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestService_Proxy_Get_And_Search(t *testing.T) {
	tenant := tenants.Test().Sole()
	orcaPublicURL := must.ParseURL("http://example.com/fhir")

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
		mockedFHIRRequestURL          *string
		mockedFHIRResponseStatusCode  *int
		expectedJSON                  string
		expectedError                 error
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
			expectedJSON:   `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: X-Scp-Context header must be set"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:           "Fails: header value is not a valid URL",
			expectedStatus: http.StatusBadRequest,
			xSCPContext:    "not-a-valid-url",
			expectedJSON:   `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: specified SCP context header does not refer to a CarePlan"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:           "Fails: header value is relative URL (missing scheme and host)",
			expectedStatus: http.StatusInternalServerError,
			xSCPContext:    "/CarePlan/123",
			expectedJSON:   ``,
		},
		{
			name:           "Fails: header value missing scheme",
			expectedStatus: http.StatusInternalServerError,
			xSCPContext:    "example.com/fhir/CarePlan/123",
			expectedJSON:   ``,
		},
		{
			name:           "Fails: header value missing host",
			expectedStatus: http.StatusInternalServerError,
			xSCPContext:    "https:///fhir/CarePlan/123",
			expectedJSON:   ``,
		},
		{
			name:           "Fails: header value has invalid scheme",
			expectedStatus: http.StatusInternalServerError,
			xSCPContext:    "ftp://example.com/fhir/CarePlan/123",
			expectedJSON:   ``,
		},
		{
			name:           "Fails: header resource is not CarePlan",
			expectedStatus: http.StatusBadRequest,
			xSCPContext:    "https://example.com/fhir/SomeResource/invalid",
			expectedJSON:   `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: specified SCP context header does not refer to a CarePlan"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CarePlan in header not found - GET",
			expectedStatus:     http.StatusNotFound,
			readStatusReturn:   http.StatusNotFound,
			readBodyReturnFile: "./testdata/careplan-not-found.json",
			xSCPContext:        "CarePlan/not-exists",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: OperationOutcome, issues: [not-found error] CarePlan not found"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CarePlan in header not found - POST",
			expectedStatus:     http.StatusNotFound,
			readStatusReturn:   http.StatusNotFound,
			readBodyReturnFile: "./testdata/careplan-not-found.json",
			xSCPContext:        "CarePlan/not-exists",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/test/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/test/fhir/Patient/_search failed: OperationOutcome, issues: [not-found error] CarePlan not found"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CareTeam not present in bundle - GET",
			expectedStatus:     http.StatusInternalServerError,
			readBodyReturnFile: "./testdata/careplan-careteam-missing.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: invalid CareTeam reference (must be a reference to a contained resource): CareTeam/cps-careteam-01"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: CareTeam not present in bundle - POST",
			expectedStatus:     http.StatusInternalServerError,
			readBodyReturnFile: "./testdata/careplan-careteam-missing.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/test/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/test/fhir/Patient/_search failed: invalid CareTeam reference (must be a reference to a contained resource): CareTeam/cps-careteam-01"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: requester not part of CareTeam - GET",
			principal:          auth.TestPrincipal3,
			expectedStatus:     http.StatusForbidden,
			readBodyReturnFile: "./testdata/careplan-valid.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:               "Fails: requester not part of CareTeam - POST",
			principal:          auth.TestPrincipal3,
			expectedStatus:     http.StatusForbidden,
			readBodyReturnFile: "./testdata/careplan-valid.json",
			readStatusReturn:   http.StatusOK,
			xSCPContext:        "CarePlan/cps-careplan-01",
			method:             to.Ptr("POST"),
			url:                to.Ptr("/cpc/test/fhir/Patient/_search"),
			expectedJSON:       `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/test/fhir/Patient/_search failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                         "Fails: proxied request returns error - GET",
			expectedStatus:               http.StatusNotFound,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRRequestURL:         to.Ptr("GET /fhir/Patient/1"),
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusNotFound),
		},
		{
			name:                         "Fails: proxied request returns error - POST",
			expectedStatus:               http.StatusNotFound,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRRequestURL:         to.Ptr("GET /fhir/Patient/1"),
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusNotFound),
			method:                       to.Ptr("POST"),
			url:                          to.Ptr("/cpc/test/fhir/Patient/_search"),
		},
		{
			name:                         "Fails: requester is CareTeam member but Period is expired - GET",
			principal:                    auth.TestPrincipal2,
			expectedStatus:               http.StatusForbidden,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRRequestURL:         to.Ptr("GET /fhir/Patient/1"),
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
			expectedJSON:                 `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/GET /cpc/test/fhir/Patient/1 failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                         "Fails: requester is CareTeam member but Period is expired - POST",
			principal:                    auth.TestPrincipal2,
			expectedStatus:               http.StatusForbidden,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRRequestURL:         to.Ptr("GET /fhir/Patient/1"),
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
			method:                       to.Ptr("POST"),
			url:                          to.Ptr("/cpc/test/fhir/Patient/_search"),
			expectedJSON:                 `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributor/POST /cpc/test/fhir/Patient/_search failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`,
		},
		{
			name:                         "Success: valid request - GET",
			expectedStatus:               http.StatusOK,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			mockedFHIRRequestURL:         to.Ptr("/Patient/1"),
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
		},
		{
			name:                         "Success: valid request - GET (search)",
			expectedStatus:               http.StatusOK,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			mockedFHIRRequestURL:         to.Ptr("/Patient"),
			url:                          to.Ptr("/cpc/test/fhir/Patient"),
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
		},
		{
			name:                         "Success: valid request - GET - Allow caching",
			expectedStatus:               http.StatusOK,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			mockedFHIRRequestURL:         to.Ptr("/Patient/1"),
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
			allowCaching:                 true,
		},
		{
			name:                         "Success: valid request - POST",
			expectedStatus:               http.StatusOK,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			mockedFHIRRequestURL:         to.Ptr("/Patient/_search"),
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
			method:                       to.Ptr("POST"),
			url:                          to.Ptr("/cpc/test/fhir/Patient/_search"),
		},
		{
			name:                         "Success: valid request - POST - Allow caching",
			expectedStatus:               http.StatusOK,
			readBodyReturnFile:           "./testdata/careplan-valid.json",
			mockedFHIRRequestURL:         to.Ptr("/Patient/_search"),
			readStatusReturn:             http.StatusOK,
			xSCPContext:                  "CarePlan/cps-careplan-01",
			mockedFHIRResponseStatusCode: to.Ptr(http.StatusOK),
			method:                       to.Ptr("POST"),
			url:                          to.Ptr("/cpc/test/fhir/Patient/_search"),
			allowCaching:                 true,
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
			carePlanServiceURL.Path = "/cps/test"

			healthDataViewEndpointEnabled := true
			if tt.healthDataViewEndpointEnabled != nil {
				healthDataViewEndpointEnabled = *tt.healthDataViewEndpointEnabled
			}

			service, _ := New(
				Config{HealthDataViewEndpointEnabled: healthDataViewEndpointEnabled},
				tenants.Test(), profile.Test(), orcaPublicURL, sessionManager,
				events.NewManager(messageBroker), true, nil)
			service.ehrFHIRProxyByTenant = map[string]coolfhir.HttpProxy{
				tenant.ID: coolfhir.NewProxy(
					"MockProxy",
					fhirServerURL,
					"/cpc/test/fhir",
					orcaPublicURL.JoinPath("/cpc/test/fhir"),
					http.DefaultTransport,
					tt.allowCaching, false,
				),
			}
			service.ehrFHIRClientByTenant = map[string]fhirclient.Client{
				tenant.ID: fhirclient.New(fhirServerURL, &http.Client{}, nil),
			}

			// Setup: configure the service to proxy to the backing FHIR server
			frontServerMux := http.NewServeMux()

			if tt.readBodyReturnFile != "" {
				carePlanServiceMux.HandleFunc(fmt.Sprintf("GET /cps/test/%s", tt.xSCPContext), func(writer http.ResponseWriter, request *http.Request) {
					rawJson, _ := os.ReadFile(tt.readBodyReturnFile)
					writer.WriteHeader(tt.readStatusReturn)
					_, _ = writer.Write(rawJson)
				})
			}
			if tt.mockedFHIRRequestURL != nil && tt.mockedFHIRResponseStatusCode != nil {
				fhirServerMux.HandleFunc(*tt.mockedFHIRRequestURL, func(writer http.ResponseWriter, request *http.Request) {
					writer.Header().Set("Content-Type", "application/fhir+json")
					writer.WriteHeader(*tt.mockedFHIRResponseStatusCode)
					if *tt.mockedFHIRResponseStatusCode == http.StatusOK {
						// Return a valid FHIR response for successful requests
						if strings.Contains(*tt.mockedFHIRRequestURL, "_search") || !strings.Contains(*tt.mockedFHIRRequestURL, "/1") {
							// For search endpoints, return a Bundle
							_, _ = writer.Write([]byte(`{
								"resourceType": "Bundle",
								"type": "searchset",
								"total": 1,
								"entry": [
									{
										"fullUrl": "http://example.com/fhir/Patient/1",
										"resource": {
											"resourceType": "Patient",
											"id": "1",
											"identifier": [
												{
													"system": "http://fhir.nl/fhir/NamingSystem/bsn",
													"value": "1333333337"
												}
											]
										}
									}
								]
							}`))
						} else {
							// For individual resource endpoints, return a Patient
							_, _ = writer.Write([]byte(`{
								"resourceType": "Patient",
								"id": "1",
								"identifier": [
									{
										"system": "http://fhir.nl/fhir/NamingSystem/bsn",
										"value": "1333333337"
									}
								]
							}`))
						}
					}
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
			reqURL := "/cpc/" + tenant.ID + "/fhir/Patient/1"
			if tt.url != nil {
				reqURL = *tt.url
			}
			slog.Info(fmt.Sprintf("Requesting %s %s", method, frontServer.URL+reqURL))
			slog.Info(fmt.Sprintf("FHIR Server URL: %s", fhirServer.URL))
			slog.Info(fmt.Sprintf("CarePlan Service URL: %s", carePlanServiceURL))

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
	orcaPublicURL := must.ParseURL("http://example.com/fhir")

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
	carePlanServiceMux.HandleFunc("GET /cps/{tenant}/Task/999", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	})
	carePlanServiceMux.HandleFunc("GET /cps/{tenant}/Task/1", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-1.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	carePlanServiceMux.HandleFunc("GET /cps/{tenant}/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-2.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	var capturedTaskUpdate fhir.Task
	carePlanServiceMux.HandleFunc("PUT /cps/{tenant}/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := io.ReadAll(request.Body)
		_ = json.Unmarshal(rawJson, &capturedTaskUpdate)
		writer.WriteHeader(http.StatusOK)
	})
	carePlanServiceMux.HandleFunc("GET /cps/{tenant}/Task/3", func(writer http.ResponseWriter, request *http.Request) {
		rawJson, _ := os.ReadFile("./testdata/task-3.json")
		var data fhir.Task
		_ = json.Unmarshal(rawJson, &data)
		responseData, _ := json.Marshal(data)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/cps/test"

	tenantCfg := tenants.Test(func(properties *tenants.Properties) {
		properties.TaskEngine = tenants.TaskEngineProperties{
			Enabled: true,
		}
	})
	service, _ := New(Config{}, tenantCfg, profile.Test(), orcaPublicURL, sessionManager, events.NewManager(messageBroker), true, nil)

	frontServerMux := http.NewServeMux()
	frontServer := httptest.NewServer(frontServerMux)
	service.RegisterHandlers(frontServerMux)
	ctx := tenants.WithTenant(context.Background(), tenantCfg.Sole())
	httpClient, _ := prof.HttpClient(ctx, auth.TestPrincipal1.Organization.Identifier[0])

	cpcBaseURL := frontServer.URL + basePath + "/test/fhir"
	t.Run("invalid notification - wrong data type", func(t *testing.T) {
		notification := fhir.Task{Id: to.Ptr("1")}
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
	})
	t.Run("valid notification (no trailing slash)", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Patient/1"), Type: to.Ptr("Patient")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification (with trailing slash)", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Patient/1"), Type: to.Ptr("Patient")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/test/fhir/", strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not found", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/999"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusInternalServerError, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - not SCP", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/1"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader(string(notificationJSON)))
		httpResponse, err := httpClient.Do(httpRequest)

		require.NoError(t, err)

		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("valid notification - task - invalid task missing focus", func(t *testing.T) {
		notification := coolfhir.CreateSubscriptionNotification(carePlanServiceURL,
			time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			fhir.Reference{Reference: to.Ptr("CareTeam/1")}, 1, fhir.Reference{Reference: to.Ptr("Task/2"), Type: to.Ptr("Task")})
		notificationJSON, _ := json.Marshal(notification)
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader(string(notificationJSON)))
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
		httpRequest, _ := http.NewRequest("POST", cpcBaseURL, strings.NewReader("invalid"))
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
	orcaPublicURL := must.ParseURL("http://example.com/fhir")

	prof := profile.TestProfile{
		Principal: auth.TestPrincipal2,
	}
	testCfg := tenants.Test(func(properties *tenants.Properties) {
		properties.TaskEngine = tenants.TaskEngineProperties{
			Enabled: true,
		}
	})
	ctx := tenants.WithTenant(context.Background(), testCfg.Sole())
	httpClient, _ := prof.HttpClient(ctx, auth.TestPrincipal2.Organization.Identifier[0])
	// Test that the service registers the /cpc URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)

	service, _ := New(
		Config{},
		testCfg, profile.TestProfile{
			Principal: auth.TestPrincipal2,
		}, orcaPublicURL, sessionManager, events.NewManager(messageBroker), true, nil)
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

	httpRequest, _ := http.NewRequest("POST", frontServer.URL+basePath+"/test/fhir", strings.NewReader(string(notificationJSON)))
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
	orcaPublicURL := must.ParseURL("http://example.com/fhir")

	clients.Factories["test"] = func(properties map[string]string) clients.ClientProperties {
		return clients.ClientProperties{
			Client:  fhirServer.Client().Transport,
			BaseURL: fhirServerURL,
		}
	}
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)
	sessionManager, sessionID := createTestSession()

	tenantCfg := tenants.Config{
		"test": tenants.Test().Sole(),
		"other": tenants.Test(func(properties *tenants.Properties) {
			properties.ID = "other"
		}).Sole(),
	}
	service, err := New(Config{}, tenantCfg, profile.Test(), orcaPublicURL, sessionManager, events.NewManager(messageBroker), true, nil)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/test/ehr/fhir/Patient/1", nil)
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

	t.Run("multi-tenancy", func(t *testing.T) {
		t.Run("request targets a different tenant than the session's", func(t *testing.T) {
			httpRequest, _ = http.NewRequest("GET", frontServer.URL+"/cpc/other/ehr/fhir/Patient/1", nil)
			httpRequest.AddCookie(&http.Cookie{
				Name:  "sid",
				Value: sessionID,
			})
			httpResponse, err = frontServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusForbidden, httpResponse.StatusCode)
			responseData, _ := io.ReadAll(httpResponse.Body)
			require.Equal(t, "session tenant does not match request tenant", strings.TrimSpace(string(responseData)))
		})
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

	httpRequest, _ = http.NewRequest("GET", frontServer.URL+"/cpc/test/ehr/fhir/Patient/1", nil)
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
	orcaPublicURL := must.ParseURL("http://example.com/fhir")

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
	}, tenants.Test(), profile.Test(), orcaPublicURL, sessionManager, events.NewManager(messageBroker), false, nil)
	require.NoError(t, err)

	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)
	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	t.Run("ok", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/test/fhir/Endpoint", nil)
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
		responseData, _ := io.ReadAll(httpResponse.Body)
		expectedData, err := os.ReadFile("./testdata/endpoints.json")
		require.NoError(t, err)
		require.JSONEq(t, string(expectedData), string(responseData))
	})
	t.Run("search parameters not allowed", func(t *testing.T) {
		httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/cpc/test/fhir/Endpoint?foo=bar", nil)
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
			TenantID:       "test",
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
		"taskIdentifier": "task-identifier-123",
		"tenantId": "test"
	}`, httpResponse.Body.String())
	})
	t.Run("no PractitionerRole", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()
		sessionData := session.Data{
			TaskIdentifier: to.Ptr("task-identifier-123"),
			TenantID:       "test",
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
		"taskIdentifier": "task-identifier-123",
		"tenantId": "test"
	}`, httpResponse.Body.String())
	})
}

func TestService_ExternalFHIRProxy(t *testing.T) {
	t.Log("This tests the external FHIR proxy functionality (/cpc/external/fhir, used by the EHR and ORCA Frontend to query remote SCP-nodes' FHIR APIs.")

	remoteFHIRAPIMux := http.NewServeMux()
	remoteFHIRAPIMux.HandleFunc("GET /fhir/Task/1", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{Id: to.Ptr("1")})
	})
	remoteSCPNode := httptest.NewServer(remoteFHIRAPIMux)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cps/test/Task/2", func(writer http.ResponseWriter, request *http.Request) {
		coolfhir.SendResponse(writer, http.StatusOK, fhir.Task{Id: to.Ptr("2")})
	})

	httpServer := httptest.NewServer(mux)
	sessionManager, sessionID := createTestSession()
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
		tenants: map[string]tenants.Properties{
			"test": tenants.Test().Sole(),
			"other": tenants.Test(func(properties *tenants.Properties) {
				properties.ID = "other"
			}).Sole(),
		},
		httpHandler:    mux,
		cpsEnabled:     true,
		orcaPublicURL:  must.ParseURL(httpServer.URL),
		config:         Config{StaticBearerToken: "secret"},
		SessionManager: sessionManager,
	}
	service.createFHIRClientForURL = service.defaultCreateFHIRClientForURL
	service.RegisterHandlers(mux)

	baseURL := httpServer.URL + "/cpc/test/external/fhir"
	t.Run("X-Scp-Entity-Identifier", func(t *testing.T) {
		httpRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/Task/1", nil)
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
			httpRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/Task/1", nil)
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
			httpRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/Task/2", nil)
			httpRequest.Header.Set("Authorization", "Bearer secret")
			httpRequest.Header.Set("X-Scp-Fhir-Url", httpServer.URL+"/cps/test")
			httpResponse, err := httpServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			responseData, err := io.ReadAll(httpResponse.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, responseData)
		})
		t.Run("local-cps", func(t *testing.T) {
			httpRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/Task/2", nil)
			httpRequest.Header.Set("Authorization", "Bearer secret")
			httpRequest.Header.Set("X-Scp-Fhir-Url", "local-cps")
			httpResponse, err := httpServer.Client().Do(httpRequest)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, httpResponse.StatusCode)
			responseData, err := io.ReadAll(httpResponse.Body)
			require.NoError(t, err)
			assert.NotEmpty(t, responseData)
		})
		t.Run("local-cps, non-root base URL", func(t *testing.T) {
			t.Log("calls to local CPS are dispatched internally, this test makes sure this also works when ORCA is running on a subpath")
			service.orcaPublicURL = must.ParseURL(httpServer.URL).JoinPath("orca")
			defer func() {
				service.orcaPublicURL = must.ParseURL(httpServer.URL)
			}()

			httpRequest, _ := http.NewRequest(http.MethodGet, baseURL+"/Task/2", nil)
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
	t.Run("multi-tenancy", func(t *testing.T) {
		t.Run("user agent is browser, user session", func(t *testing.T) {
			t.Run("request targets a different tenant than the session's", func(t *testing.T) {
				// User session is for tenant "test", request is for tenant "other"
				httpRequest, _ := http.NewRequest(http.MethodGet, httpServer.URL+"/cpc/other/external/fhir", nil)
				httpRequest.Header.Set("X-Scp-Fhir-Url", remoteSCPNode.URL+"/fhir")
				httpRequest.AddCookie(&http.Cookie{
					Name:  "sid",
					Value: sessionID,
				})
				httpResponse, err := httpServer.Client().Do(httpRequest)
				require.NoError(t, err)
				require.Equal(t, http.StatusForbidden, httpResponse.StatusCode)
				responseData, err := io.ReadAll(httpResponse.Body)
				require.NoError(t, err)
				assert.Equal(t, "session tenant does not match request tenant", strings.TrimSpace(string(responseData)))
			})
		})
	})
	t.Run("can't determine remote node", func(t *testing.T) {
		httpRequest, _ := http.NewRequest(http.MethodPost, baseURL+"/Task/_search", nil)
		httpRequest.Header.Set("Authorization", "Bearer secret")
		httpResponse, err := httpServer.Client().Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, httpResponse.StatusCode)
		responseData, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err)
		assert.Contains(t, string(responseData), "can't determine the external SCP-node to query from the HTTP request headers")
	})
}

func TestService_Metadata(t *testing.T) {
	tenant := tenants.Test().Sole()
	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	service := &Service{
		profile: profile.Test(),
		tenants: tenants.Test(),
	}
	service.RegisterHandlers(mux)

	httpResponse, err := http.Get(httpServer.URL + "/cpc/" + tenant.ID + "/fhir/metadata")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	responseData, err := io.ReadAll(httpResponse.Body)
	require.NoError(t, err)
	var capabilityStatement fhir.CapabilityStatement
	require.NoError(t, json.Unmarshal(responseData, &capabilityStatement))
	assert.Contains(t, string(responseData), "SMART-on-FHIR") // set by profile
}

func TestService_Import(t *testing.T) {
	tenant := tenants.Test().Sole()
	mux := http.NewServeMux()
	var capturedBundle fhir.Bundle
	mux.HandleFunc("/cps/"+tenant.ID+"/$import", func(w http.ResponseWriter, r *http.Request) {
		capturedBundleJSON, _ := io.ReadAll(r.Body)
		println(string(capturedBundleJSON))
		_ = json.Unmarshal(capturedBundleJSON, &capturedBundle)
		coolfhir.SendResponse(w, http.StatusOK, fhir.Bundle{})
	})
	httpServer := httptest.NewServer(mux)
	ehrFHIRClient := &test.StubFHIRClient{
		Resources: []any{
			fhir.Patient{
				Id: to.Ptr("1"),
				Identifier: []fhir.Identifier{
					{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
		},
	}

	service := &Service{
		profile: profile.TestProfile{
			Principal: auth.TestPrincipal1,
		},
		tenants: map[string]tenants.Properties{
			tenant.ID:      tenant,
			"other_tenant": {},
		},
		httpHandler:   mux,
		cpsEnabled:    true,
		orcaPublicURL: must.ParseURL(httpServer.URL),
		config:        Config{StaticBearerToken: "secret"},
		ehrFHIRClientByTenant: map[string]fhirclient.Client{
			tenant.ID: ehrFHIRClient,
		},
	}
	service.RegisterHandlers(mux)
	httpClient := http.Client{
		Transport: auth.AuthenticatedTestRoundTripper(http.DefaultTransport, auth.TestPrincipal2, ""),
	}

	t.Run("ok - Demo EHR", func(t *testing.T) {
		t.Log("In this test, test org 2 imports data into org 1's CPS")
		cpsFHIRClient := &test.StubFHIRClient{}
		globals.RegisterCPSFHIRClient(tenant.ID, cpsFHIRClient)

		start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		requestBody := fhir.Parameters{
			Parameter: []fhir.ParametersParameter{
				{
					Name: "patient",
					ValueIdentifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
				{
					Name: "servicerequest",
					ValueCoding: &fhir.Coding{
						System:  to.Ptr("http://example.com/servicerequest"),
						Code:    to.Ptr("sr1"),
						Display: to.Ptr("ServiceRequestDisplay"),
					},
				},
				{
					Name: "condition",
					ValueCoding: &fhir.Coding{
						System:  to.Ptr("http://example.com/condition"),
						Code:    to.Ptr("c1"),
						Display: to.Ptr("ConditionDisplay"),
					},
				},
				{
					Name:          "start",
					ValueDateTime: to.Ptr(start.Format(time.RFC3339)),
				},
			},
		}
		requestBodyJSON := must.MarshalJSON(requestBody)
		println(string(requestBodyJSON))
		httpRequest, _ := http.NewRequest("POST", httpServer.URL+"/cpc/test/fhir/$import", bytes.NewReader(requestBodyJSON))
		httpRequest.Header.Set("Content-Type", "application/fhir+json")
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	})
	t.Run("ok - zorgplatform", func(t *testing.T) {
		t.Log("In this test, test org 2 imports data into org 1's CPS")
		cpsFHIRClient := &test.StubFHIRClient{}
		globals.RegisterCPSFHIRClient(tenant.ID, cpsFHIRClient)

		start := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		requestBody := fhir.Parameters{
			Parameter: []fhir.ParametersParameter{
				{
					Name: "patient",
					ValueIdentifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
				{
					Name: "servicerequest",
					ValueCoding: &fhir.Coding{
						System:  to.Ptr("http://example.com/servicerequest"),
						Code:    to.Ptr("sr1"),
						Display: to.Ptr("ServiceRequestDisplay"),
					},
				},
				{
					Name: "condition",
					ValueCoding: &fhir.Coding{
						System:  to.Ptr("http://example.com/condition"),
						Code:    to.Ptr("c1"),
						Display: to.Ptr("ConditionDisplay"),
					},
				},
				{
					Name: "chipsoft_zorgplatform_workflowid",
					ValueIdentifier: &fhir.Identifier{
						System: to.Ptr("http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
						Value:  to.Ptr("workflow-123"),
					},
				},
				{
					Name:          "start",
					ValueDateTime: to.Ptr(start.Format(time.RFC3339)),
				},
			},
		}
		requestBodyJSON := must.MarshalJSON(requestBody)
		println(string(requestBodyJSON))
		httpRequest, _ := http.NewRequest("POST", httpServer.URL+"/cpc/test/fhir/$import", bytes.NewReader(requestBodyJSON))
		httpRequest.Header.Set("Content-Type", "application/fhir+json")
		httpResponse, err := httpClient.Do(httpRequest)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, httpResponse.StatusCode)

		// Assert sent bundle
		t.Run("assert sent bundle", func(t *testing.T) {
			expectedBundleJSON, err := os.ReadFile("importer/test/expected-bundle.json")
			require.NoError(t, err)
			var expectedBundle fhir.Bundle
			require.NoError(t, json.Unmarshal(expectedBundleJSON, &expectedBundle))
			for _, capturedEntry := range capturedBundle.Entry {
				resourceType := capturedEntry.Request.Url
				var capturedResource map[string]any
				require.NoError(t, json.Unmarshal(capturedEntry.Resource, &capturedResource))

				// Find resourceType in expectedBundle, unmarshal, compare
				var expectedResources []map[string]any
				err := coolfhir.ResourcesInBundle(&expectedBundle, coolfhir.EntryIsOfType(resourceType), &expectedResources)
				require.NoError(t, err, "resource %s not found in expected bundle", resourceType)

				diffs := deep.Equal(expectedResources[0], capturedResource)
				for _, diff := range diffs {
					println("Diff for", resourceType, ":", diff)
				}
				assert.Empty(t, diffs, "resource %s does not match expected", resourceType)
			}
		})
	})
	t.Run("$import operation not enabled for this tenant", func(t *testing.T) {
		httpResponse, err := httpClient.PostForm(httpServer.URL+"/cpc/other_tenant/fhir/$import", url.Values{
			"patient_identifier": []string{"123456789"},
		})
		require.NoError(t, err)
		responseData, _ := io.ReadAll(httpResponse.Body)
		assert.Contains(t, string(responseData), "not enabled")
		require.Equal(t, http.StatusForbidden, httpResponse.StatusCode)
	})
}

func TestService_withSessionOrBearerToken(t *testing.T) {
	// Generate a test token generator
	tokenGenerator, err := rp.NewTestTokenGenerator()
	require.NoError(t, err)

	// Generate a valid token
	validToken, err := tokenGenerator.CreateToken(nil)
	require.NoError(t, err)

	// Generate an invalid token (expired)
	invalidToken, err := tokenGenerator.CreateExpiredToken()
	require.NoError(t, err)

	// Create a mock token client
	ctx := context.Background()
	mockClient, err := rp.NewMockClient(ctx, tokenGenerator)
	require.NoError(t, err)

	// Make sure globals.StrictMode is false for the tests
	origStrictMode := globals.StrictMode
	globals.StrictMode = false
	defer func() {
		globals.StrictMode = origStrictMode
	}()

	tests := []struct {
		name           string
		requestSetup   func(req *http.Request)
		staticToken    string
		expectedStatus int
		withMockClient bool
		strictMode     bool
	}{
		{
			name: "With valid session",
			requestSetup: func(req *http.Request) {
				// Session cookie is added by the test setup
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "With static bearer token when strict mode is off",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer static-token")
			},
			staticToken:    "static-token",
			expectedStatus: http.StatusOK,
		},
		{
			name: "With valid ADB2C token",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+validToken)
			},
			withMockClient: true,
			expectedStatus: http.StatusOK,
		},
		{
			name: "With invalid ADB2C token",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+invalidToken)
			},
			withMockClient: true,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "With no authorization",
			requestSetup: func(req *http.Request) {
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "With empty bearer token",
			requestSetup: func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer ")
			},
			withMockClient: true,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original strict mode and restore after test
			origMode := globals.StrictMode
			globals.StrictMode = tt.strictMode
			defer func() {
				globals.StrictMode = origMode
			}()

			// Setup: Create a session manager with a test session
			sessionManager, sessionCookie := createTestSession()

			// Setup: Create the service with the session manager
			service := &Service{
				SessionManager: sessionManager,
				config:         Config{StaticBearerToken: tt.staticToken},
				tokenClient:    nil,
				tenants:        tenants.Test(),
			}
			if tt.withMockClient {
				service.tokenClient = mockClient.Client
			}

			// Setup: Create a handler that will be called if authentication is successful
			handlerCalled := false
			handler := http.NewServeMux()
			// Call the middleware with our handler
			handler.HandleFunc("/{tenant}", service.tenants.HttpHandler(service.withUserAuth(func(res http.ResponseWriter, req *http.Request) {
				handlerCalled = true
				res.WriteHeader(http.StatusOK)
			})))

			// Create a test HTTP server with the wrapped handler
			testServer := httptest.NewServer(handler)
			defer testServer.Close()

			// Create a test request
			req, err := http.NewRequest("GET", testServer.URL+"/test", nil)
			require.NoError(t, err)

			// Add a session cookie if the test case needs it
			if tt.name == "With valid session" {
				// Set the correct cookie name and format from the createTestSession function
				req.AddCookie(&http.Cookie{
					Name:  "sid", // This should match what's used in the session manager
					Value: sessionCookie,
				})
			}

			// Apply any custom request setup
			tt.requestSetup(req)

			// Make the request
			resp, err := testServer.Client().Do(req) // Use the test server's client to ensure cookies are handled properly
			require.NoError(t, err)
			defer resp.Body.Close()

			// Check the response status code
			assert.Equal(t, tt.expectedStatus, resp.StatusCode, "Unexpected status code for test: %s", tt.name)

			// Verify if the handler was called when expected
			if tt.expectedStatus == http.StatusOK {
				assert.True(t, handlerCalled, "Handler should have been called for successful auth")
			} else {
				assert.False(t, handlerCalled, "Handler should not have been called for failed auth")
			}
		})
	}
}

func createTestSession() (*user.SessionManager[session.Data], string) {
	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	sessionHttpResponse := httptest.NewRecorder()
	sessionManager.Create(sessionHttpResponse, session.Data{
		FHIRLauncher: "test",
		TenantID:     tenants.Test().Sole().ID,
	})
	// extract session ID; sid=<something>;
	cookieValue := sessionHttpResponse.Header().Get("Set-Cookie")
	cookieValue = strings.Split(cookieValue, ";")[0]
	cookieValue = strings.Split(cookieValue, "=")[1]
	return sessionManager, cookieValue
}
