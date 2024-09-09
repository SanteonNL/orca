package careplanservice

import (
	"encoding/json"
	"io"
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
var nutsPublicURL, _ = url.Parse("https://example.com/nuts")

var taskJSON = "{\"resourceType\":\"Task\",\"id\":\"cps-task-01\",\"meta\":{\"versionId\":\"1\",\"profile\":[\"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask\"]},\"text\":{\"status\":\"generated\",\"div\":\"<divxmlns=\\\"http://www.w3.org/1999/xhtml\\\"><pclass=\\\"res-header-id\\\"><b>GeneratedNarrative:Taskcps-task-01</b></p><aname=\\\"cps-task-01\\\"></a><aname=\\\"hccps-task-01\\\"></a><aname=\\\"cps-task-01-en-US\\\"></a><divstyle=\\\"display:inline-block;background-color:#d9e0e7;padding:6px;margin:4px;border:1pxsolid#8da1b4;border-radius:5px;line-height:60%\\\"><pstyle=\\\"margin-bottom:0px\\\">version:1</p><pstyle=\\\"margin-bottom:0px\\\">Profile:<ahref=\\\"StructureDefinition-SCPTask.html\\\">SharedCarePlanning:TaskProfile</a></p></div><p><b>status</b>:Requested</p><p><b>intent</b>:order</p><p><b>code</b>:<spantitle=\\\"Codes:{http://hl7.org/fhir/CodeSystem/task-codefullfill}\\\">fullfill</span></p><p><b>focus</b>:<ahref=\\\"Bundle-cps-bundle-01.html#urn-uuid-456\\\">Bundle:type=transaction</a></p><p><b>for</b>:Identifier:<code>http://fhir.nl/fhir/NamingSystem/bsn</code>/111222333</p><p><b>requester</b>:Identifier:<code>http://fhir.nl/fhir/NamingSystem/uzi</code>/UZI-1</p><p><b>owner</b>:Identifier:<code>http://fhir.nl/fhir/NamingSystem/ura</code>/URA-2</p><p><b>reasonReference</b>:<ahref=\\\"Bundle-cps-bundle-01.html#urn-uuid-789\\\">Bundle:type=transaction</a></p></div>\"},\"status\":\"requested\",\"intent\":\"order\",\"code\":{\"coding\":[{\"system\":\"http://hl7.org/fhir/CodeSystem/task-code\",\"code\":\"fullfill\"}]},\"focus\":{\"reference\":\"urn:uuid:456\"},\"for\":{\"identifier\":{\"system\":\"http://fhir.nl/fhir/NamingSystem/bsn\",\"value\":\"111222333\"}},\"requester\":{\"identifier\":{\"system\":\"http://fhir.nl/fhir/NamingSystem/uzi\",\"value\":\"UZI-1\"}},\"owner\":{\"identifier\":{\"system\":\"http://fhir.nl/fhir/NamingSystem/ura\",\"value\":\"URA-2\"}},\"reasonReference\":{\"reference\":\"urn:uuid:789\"}}"

func TestService_Proxy(t *testing.T) {
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)

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
		FHIR: FHIRConfig{
			BaseURL: fhirServer.URL + "/fhir",
		},
	}, nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "", nil)
	require.NoError(t, err)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpClient := frontServer.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(frontServer.Client().Transport, "")

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
			"POST",
			"/cps/Task",
			"",
			http.StatusBadRequest,
			"CarePlanService/CreateTask failed: invalid Task: unexpected end of JSON input",
		},
		{
			"PUT",
			"/cps/Task/no-such-task",
			"",
			http.StatusBadRequest,
			"CarePlanService/UpdateTask failed: invalid Task: unexpected end of JSON input",
		},
		{
			"PUT",
			"/cps/Task/no-such-task",
			taskJSON,
			http.StatusBadRequest,
			"CarePlanService/UpdateTask failed: failed to read Task: FHIR request failed (GET http://example.com/Task/no-such-task, status=500)",
		},
		{
			"POST",
			"/cps/CarePlan",
			"",
			http.StatusBadRequest,
			"CarePlanService/CreateCarePlan failed: invalid fhir.CarePlan: unexpected end of JSON input",
		},
	}

	// Setup: configure the service
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)
	service, err := New(
		Config{
			FHIR: FHIRConfig{
				BaseURL: "http://example.com",
			},
		},
		nutsPublicURL,
		orcaPublicURL,
		tokenIntrospectionEndpoint,
		"did:web:example.com",
		nil)
	require.NoError(t, err)
	serverMux := http.NewServeMux()
	service.RegisterHandlers(serverMux)
	server := httptest.NewServer(serverMux)

	// Setup: configure the client
	httpClient := server.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(server.Client().Transport, "")

	for _, tt := range tests {
		// Make an invalid call (not providing JSON payload)
		request, err := http.NewRequest(tt.method, server.URL+tt.path, strings.NewReader(tt.body))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/fhir+json")

		httpResponse, err := httpClient.Do(request)
		require.NoError(t, err)

		// Test response
		require.Equal(t, tt.expectedStatusCode, httpResponse.StatusCode)

		defer func(Body io.ReadCloser) {
			err := Body.Close()
			require.NoError(t, err)
		}(httpResponse.Body)

		var target OperationOutcomeWithResourceType
		err = json.NewDecoder(httpResponse.Body).Decode(&target)
		require.NoError(t, err)
		require.Equal(t, "OperationOutcome", *target.ResourceType)

		require.NotNil(t, target)
		require.NotEmpty(t, target.Issue)
		require.Equal(t, tt.expectedMessage, *target.Issue[0].Diagnostics)
	}
}

func Test_HandleProtectedResourceMetadata(t *testing.T) {
	// Test that the service handles the protected resource metadata URL
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)
	// Setup: configure the service
	service, err := New(Config{
		FHIR: FHIRConfig{
			BaseURL: "http://example.com",
		},
	}, nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "did:web:example.com", nil)
	require.NoError(t, err)
	// Setup: configure the service to handle the protected resource metadata URL
	serverMux := http.NewServeMux()
	service.RegisterHandlers(serverMux)
	server := httptest.NewServer(serverMux)

	httpResponse, err := server.Client().Get(server.URL + "/cps/.well-known/oauth-protected-resource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)

}

func TestNew(t *testing.T) {
	t.Run("unknown FHIR server auth type", func(t *testing.T) {
		_, err := New(Config{
			FHIR: FHIRConfig{
				BaseURL: "http://example.com",
				Auth:    FHIRAuthConfig{Type: "foo"},
			},
		}, nutsPublicURL, orcaPublicURL, nil, "", nil)
		require.EqualError(t, err, "invalid FHIR authentication type: foo")
	})
}
