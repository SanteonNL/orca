package careplancontributor

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")
var nutsPublicURL, _ = url.Parse("https://example.com/nuts")

func TestService_Proxy(t *testing.T) {
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)

	// Test that the service registers the /contrib URL that proxies to the backing FHIR server
	// Setup: configure backing FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	sessionManager, _ := createTestSession()

	service, _ := New(Config{
		FHIR: coolfhir.ClientConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "", sessionManager, http.DefaultClient, nil)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, "")

	httpResponse, err := httpClient.Get(frontServer.URL + "/contrib/fhir/Patient")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)
}

func TestService_ProxyToEHR(t *testing.T) {
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)

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

	service, err := New(Config{},
		nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "",
		sessionManager, http.DefaultClient, nil)
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
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)

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
	}, nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "",
		sessionManager, carePlanService.Client(), nil)
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

// setupAuthorizationServer starts a test OAuth2 authorization server and returns its OAuth2 Token Introspection URL.
func setupAuthorizationServer(t *testing.T) *url.URL {
	authorizationServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestData, _ := io.ReadAll(request.Body)
		switch string(requestData) {
		case "token=valid":
			writer.Header().Set("Content-Type", "application/json")
			responseData, _ := json.Marshal(map[string]interface{}{
				"active":            true,
				"organization_ura":  "1",
				"organization_name": "Hospital",
				"organization_city": "CareTown",
			})
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(responseData)
		default:
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
	}))
	t.Cleanup(func() {
		authorizationServer.Close()
	})
	u, _ := url.Parse(authorizationServer.URL)
	return u
}
