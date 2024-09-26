package careplancontributor

import (
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")

func TestService_Proxy_NoHeader_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, "")

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusBadRequest)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: X-Scp-Context header value must be set"}],"resourceType":"OperationOutcome"}`, string(body))
}

func TestService_Proxy_NoCarePlanInHeader_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	carePlanServiceMux := http.NewServeMux()
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, fmt.Sprintf("%s/SomeResource/invalid", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusBadRequest)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: specified SCP context header does not refer to a CarePlan"}],"resourceType":"OperationOutcome"}`, string(body))
}

func TestService_Proxy_CarePlanNotFound_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
		rawJson, _ := os.ReadFile("./testdata/careplan-bundle-not-found.json")
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, fmt.Sprintf("%s/CarePlan/not-exists", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusNotFound)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: CarePlan not found"}],"resourceType":"OperationOutcome"}`, string(body))
	require.Equal(t, "/cps/CarePlan?_id=not-exists&_include=CarePlan%3Acare-team", capturedURL)
}

// There is no care team present in the care plan, the proxy is not reached
func TestService_Proxy_CareTeamNotPresent_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
		rawJson, _ := os.ReadFile("./testdata/careplan-bundle-careteam-missing.json")
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusNotFound)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: CareTeam not found in bundle"}],"resourceType":"OperationOutcome"}`, string(body))
	require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedURL)
}

// The requester is not in the returned care team, the proxy is not reached
func TestService_Proxy_RequesterNotInCareTeam_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal3, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusForbidden)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: requester does not have access to resource"}],"resourceType":"OperationOutcome"}`, string(body))
	require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedURL)
}

func TestService_Proxy_Valid(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusOK)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, "", string(body))
	require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedURL)
}

// All validation succeeds but the proxied method returns an error
func TestService_Proxy_ProxyReturnsError_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal1, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusNotFound)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, "", string(body))
	require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedURL)
}

// The practitioner is in the CareTeam, but their Period is expired
func TestService_Proxy_CareTeamMemberInvalidPeriod_Fails(t *testing.T) {
	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	capturedURL := ""
	carePlanServiceMux := http.NewServeMux()
	carePlanServiceMux.HandleFunc("GET /cps/*", func(writer http.ResponseWriter, request *http.Request) {
		capturedURL = request.URL.String()
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
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, auth.TestPrincipal2, fmt.Sprintf("%s/CarePlan/cps-careplan-01", carePlanServiceURL.String()))

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/fhir/Patient", nil)
	httpResponse, err := httpClient.Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, httpResponse.StatusCode, http.StatusBadRequest)
	body, _ := io.ReadAll(httpResponse.Body)
	require.Equal(t, `{"issue":[{"severity":"error","code":"processing","diagnostics":"CarePlanContributer/GET /contrib/fhir/Patient failed: CareTeamParticipant end date is in the past"}],"resourceType":"OperationOutcome"}`, string(body))
	require.Equal(t, "/cps/CarePlan?_id=cps-careplan-01&_include=CarePlan%3Acare-team", capturedURL)
}

func TestService_ProxyToEHR(t *testing.T) {
	// Test that the service registers the EHR FHIR proxy URL that proxies to the backing FHIR server of the EHR
	// Setup: configure backing EHR FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
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

	service, err := New(Config{}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/ehr/fhir/Patient", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)
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
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/fhir"

	sessionManager, sessionID := createTestSession()

	service, err := New(Config{
		CarePlanService: CarePlanServiceConfig{
			URL: carePlanServiceURL.String(),
		},
	}, profile.TestProfile{}, orcaPublicURL, sessionManager)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the upstream CarePlanService
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/cps/fhir/Patient?_search=foo:bar", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, carePlanServiceURL.Host, capturedHost)
	require.Equal(t, "foo:bar", capturedQueryParams.Get("_search"))
}

func TestService_handleGetContext(t *testing.T) {
	httpResponse := httptest.NewRecorder()
	Service{}.handleGetContext(httpResponse, nil, &user.SessionData{
		Values: map[string]string{
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
	sessionManager := user.NewSessionManager()
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
