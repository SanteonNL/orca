package careplanservice

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
)

var orcaPublicURL, _ = url.Parse("https://example.com/orca")

var taskJSON = `{"resourceType":"Task","id":"cps-task-01","meta":{"versionId":"1","profile":["http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"]},"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">Generated Narrative</div>"},"status":"requested","intent":"order","code":{"coding":[{"system":"http://hl7.org/fhir/CodeSystem/task-code","code":"fullfill"}]},"focus":{"reference":"urn:uuid:456"},"for":{"identifier":{"system":"http://fhir.nl/fhir/NamingSystem/bsn","value":"111222333"}},"requester":{"identifier":{"system":"http://fhir.nl/fhir/NamingSystem/uzi","value":"UZI-1"}},"owner":{"identifier":{"system":"http://fhir.nl/fhir/NamingSystem/ura","value":"URA-2"}},"reasonReference":{"reference":"urn:uuid:789"}}`

func TestService_Proxy(t *testing.T) {
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

// Test invalid requests return OperationOutcome
func TestService_Post_Task_Error(t *testing.T) {
	var tests = []struct {
		method             string
		path               string
		body               string
		expectedStatusCode int
		expectedMessage    string
	}{
		{
			http.MethodPost,
			"/cps/Task",
			"",
			http.StatusBadRequest,
			"CarePlanService/CreateTask failed: invalid Task: unexpected end of JSON input",
		},
		{
			http.MethodPut,
			"/cps/Task/no-such-task",
			"",
			http.StatusBadRequest,
			"CarePlanService/UpdateTask failed: invalid Task: unexpected end of JSON input",
		},
		{
			http.MethodPut,
			"/cps/Task/no-such-task",
			taskJSON,
			http.StatusBadRequest,
			"CarePlanService/UpdateTask failed: failed to read Task: FHIR request failed (GET http://example.com/Task/no-such-task, status=500)",
		},
		{
			http.MethodPost,
			"/cps/CarePlan",
			"",
			http.StatusBadRequest,
			"CarePlanService/CreateCarePlan failed: invalid fhir.CarePlan: unexpected end of JSON input",
		},
	}

	// Setup: configure the service
	service, err := New(
		Config{
			FHIR: coolfhir.ClientConfig{
				BaseURL: "http://example.com",
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

	for _, tt := range tests {
		// Make an invalid call (not providing JSON payload)
		request, err := http.NewRequest(tt.method, server.URL+tt.path, strings.NewReader(tt.body))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/fhir+json")

		httpResponse, err := httpClient.Do(request)
		require.NoError(t, err)

		// Test response
		require.Equal(t, tt.expectedStatusCode, httpResponse.StatusCode)
		require.Equal(t, "application/fhir+json", httpResponse.Header.Get("Content-Type"))

		var target OperationOutcomeWithResourceType
		err = json.NewDecoder(httpResponse.Body).Decode(&target)
		require.NoError(t, err)
		require.Equal(t, "OperationOutcome", *target.ResourceType)

		require.NotNil(t, target)
		require.NotEmpty(t, target.Issue)
		require.Equal(t, tt.expectedMessage, *target.Issue[0].Diagnostics)
	}
}
