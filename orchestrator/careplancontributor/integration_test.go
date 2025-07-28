package careplancontributor

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
	"strings"
)

var notificationCounter = new(atomic.Int32)
var fhirBaseURL *url.URL
var httpService *httptest.Server

func Test_Integration_CPCFHIRProxy(t *testing.T) {
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanServiceURL, httpService, cpcURL := setupIntegrationTest(t, notificationEndpoint)

	dataHolderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal1, "")
	invalidCareplanTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/999")
	noXSCPHeaderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, "")

	cpsDataHolder := fhirclient.New(carePlanServiceURL, &http.Client{Transport: dataHolderTransport}, nil)

	// Create Patient that Task will be related to
	var patient fhir.Patient
	t.Log("Creating Patient")
	{
		patient = fhir.Patient{
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Telecom: []fhir.ContactPoint{
				{
					System: to.Ptr(fhir.ContactPointSystemPhone),
					Value:  to.Ptr("+31612345678"),
				},
				{
					System: to.Ptr(fhir.ContactPointSystemEmail),
					Value:  to.Ptr("test@test.com"),
				},
			},
		}
		err := cpsDataHolder.Create(patient, &patient)
		require.NoError(t, err)
	}

	var carePlan fhir.CarePlan
	var task fhir.Task
	t.Log("Creating Task")
	{
		task = fhir.Task{
			Status:    fhir.TaskStatusRequested,
			Intent:    "order",
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
		}

		err := cpsDataHolder.Create(task, &task)
		require.NoError(t, err)
		err = cpsDataHolder.Read(*task.BasedOn[0].Reference, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, task.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 1)
			require.Equal(t, "Task", *carePlan.Activity[0].Reference.Type)
			require.Equal(t, "Task/"+*task.Id, *carePlan.Activity[0].Reference.Reference)
		})
		t.Run("Search for task by ID", func(t *testing.T) {
			var fetchedBundle fhir.Bundle
			err := cpsDataHolder.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
			require.NoError(t, err)
			require.Len(t, fetchedBundle.Entry, 1)
		})
	}

	cpsDataRequester := fhirclient.New(carePlanServiceURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)
	cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)

	t.Log("Read data from EHR before Task is accepted - Fails")
	{
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Accepting Task")
	{
		task.Status = fhir.TaskStatusAccepted
		var updatedTask fhir.Task
		err := cpsDataRequester.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusAccepted, updatedTask.Status)
		})

		// Getting patient
		var fetchedPatient fhir.Patient
		err = cpsDataRequester.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.Equal(t, updatedTask.For.Identifier.System, fetchedPatient.Identifier[0].System)
		require.Equal(t, updatedTask.For.Identifier.Value, fetchedPatient.Identifier[0].Value)
		require.Equal(t, *updatedTask.For.Reference, "Patient/"+*fetchedPatient.Id)
	}
	t.Log("Read data from EHR after Task is accepted")
	{
		var fetchedTask fhir.Task

		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.NoError(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.NoError(t, err)
		require.Len(t, fetchedBundle.Entry, 1)
	}
	t.Log("Read data from EHR after Task is accepted")
	{
		requestBundle := fhir.Bundle{
			Type: fhir.BundleTypeBatch,
			Entry: []fhir.BundleEntry{
				{
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbGET,
						Url:    "Task/" + *task.Id,
					},
				},
			},
		}
		var responseBundle fhir.Bundle
		err := cpcDataRequester.Create(requestBundle, &responseBundle, fhirclient.AtPath("/"))
		require.NoError(t, err)
		require.Len(t, responseBundle.Entry, 1)
		require.NotNil(t, responseBundle.Entry[0].Response)
		require.Equal(t, "200 OK", responseBundle.Entry[0].Response.Status)
	}
	t.Log("Reading task after accepted - header references non-existent careplan - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: invalidCareplanTransport}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
	t.Log("Reading task after accepted - no xSCP header - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: noXSCPHeaderTransport}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
	t.Log("Reading task after accepted - invalid principal - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal3, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
}

func Test_Integration_JWTValidationAndExternalEndpoint(t *testing.T) {
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanServiceURL, _, _ := setupIntegrationTest(t, notificationEndpoint)

	// Setup mock external endpoint that will be called after JWT validation
	var externalEndpointCalled bool
	var receivedRequestBody string
	mockExternalEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalEndpointCalled = true

		// Read the request body
		body, _ := io.ReadAll(r.Body)
		receivedRequestBody = string(body)

		// Return a mock FHIR Bundle response
		mockBundle := fhir.Bundle{
			Type: fhir.BundleTypeSearchset,
			Entry: []fhir.BundleEntry{
				{
					Resource: json.RawMessage(`{
						"resourceType": "Patient",
						"id": "test-patient-123",
						"identifier": [
							{
								"system": "http://fhir.nl/fhir/NamingSystem/bsn",
								"value": "1333333337"
							}
						]
					}`),
				},
			},
		}

		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockBundle)
	}))
	defer mockExternalEndpoint.Close()

	// Create a test token generator for JWT validation
	tokenGen, err := rp.NewTestTokenGenerator()
	require.NoError(t, err)

	// Create a mock token client
	ctx := context.Background()
	mockTokenClient, err := rp.NewMockClient(ctx, tokenGen)
	require.NoError(t, err)

	// Setup CPC service with JWT validation enabled
	cpcConfig := DefaultConfig()
	cpcConfig.Enabled = true
	cpcConfig.FHIR.BaseURL = fhirBaseURL.String()
	cpcConfig.HealthDataViewEndpointEnabled = true
	cpcConfig.OIDC.RelyingParty.Enabled = true
	cpcConfig.OIDC.RelyingParty.ClientID = tokenGen.ClientID
	cpcConfig.OIDC.RelyingParty.TrustedIssuers = map[string]rp.TrustedIssuer{
		"test": {
			IssuerURL:    tokenGen.GetIssuerURL(),
			DiscoveryURL: "https://mock-discovery.example.com/.well-known/openid_configuration",
		},
	}

	sessionManager, _ := createTestSession()
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)

	cpc, err := New(cpcConfig, tenants.Test(), profile.TestProfile{}, orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), nil, nil, carePlanServiceURL, nil)
	require.NoError(t, err)

	cpc.tokenClient = mockTokenClient.Client

	// Setup HTTP server for CPC
	cpcServerMux := http.NewServeMux()
	cpcHttpService := httptest.NewServer(cpcServerMux)
	defer cpcHttpService.Close()
	cpc.RegisterHandlers(cpcServerMux)

	t.Run("Valid JWT token allows access to external endpoint", func(t *testing.T) {
		externalEndpointCalled = false
		receivedRequestBody = ""

		// Create a valid JWT token
		validToken, err := tokenGen.CreateToken(map[string]interface{}{
			"sub":   "test-user-123",
			"name":  "Test User",
			"email": "test@example.com",
			"roles": []string{"User", "Admin"},
		})
		require.NoError(t, err)

		// Create HTTP client with JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint with JWT token
		req, err := http.NewRequest("GET", cpcHttpService.URL+"/cpc/external/fhir/Patient/test-patient-123", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validToken)
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was successful
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the external endpoint was called
		require.True(t, externalEndpointCalled, "External endpoint should have been called")

		// Verify the response contains expected FHIR data
		var responseBundle fhir.Bundle
		err = json.NewDecoder(resp.Body).Decode(&responseBundle)
		require.NoError(t, err)
		require.Equal(t, fhir.BundleTypeSearchset, responseBundle.Type)
		require.Len(t, responseBundle.Entry, 1)

		// Verify the patient data in the response by parsing the JSON
		var patient fhir.Patient
		err = json.Unmarshal(responseBundle.Entry[0].Resource, &patient)
		require.NoError(t, err)
		require.Equal(t, "test-patient-123", *patient.Id)
	})

	t.Run("Invalid JWT token denies access", func(t *testing.T) {
		externalEndpointCalled = false

		// Create an invalid (expired) JWT token
		expiredToken, err := tokenGen.CreateExpiredToken()
		require.NoError(t, err)

		// Create HTTP client with expired JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint with expired JWT token
		req, err := http.NewRequest("GET", cpcHttpService.URL+"/cpc/external/fhir/Patient/test-patient-123", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was denied
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Verify the external endpoint was NOT called
		require.False(t, externalEndpointCalled, "External endpoint should not have been called with invalid token")
	})

	t.Run("Malformed JWT token denies access", func(t *testing.T) {
		externalEndpointCalled = false

		// Create HTTP client with malformed JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint with malformed JWT token
		req, err := http.NewRequest("GET", cpcHttpService.URL+"/cpc/external/fhir/Patient/test-patient-123", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid.malformed.token")
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was denied
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Verify the external endpoint was NOT called
		require.False(t, externalEndpointCalled, "External endpoint should not have been called with malformed token")
	})

	t.Run("Missing JWT token denies access", func(t *testing.T) {
		externalEndpointCalled = false

		// Create HTTP client without JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint without JWT token
		req, err := http.NewRequest("GET", cpcHttpService.URL+"/cpc/external/fhir/Patient/test-patient-123", nil)
		require.NoError(t, err)
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was denied
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		// Verify the external endpoint was NOT called
		require.False(t, externalEndpointCalled, "External endpoint should not have been called without token")
	})

	t.Run("Valid JWT token with custom claims", func(t *testing.T) {
		externalEndpointCalled = false

		// Create a valid JWT token with custom claims
		customClaims := map[string]interface{}{
			"sub":                       "healthcare-provider-456",
			"name":                      "Dr. Jane Smith",
			"email":                     "jane.smith@hospital.com",
			"roles":                     []string{"Doctor", "Specialist"},
			"organization":              "Hospital ABC",
			"extension_CustomAttribute": "custom-value",
			"groups": []string{
				"cardiology-department",
				"senior-staff",
			},
		}

		validToken, err := tokenGen.CreateToken(customClaims)
		require.NoError(t, err)

		// Validate the token to ensure it contains the expected claims
		claims, err := mockTokenClient.ValidateToken(ctx, validToken, rp.WithValidateSignature(false))
		require.NoError(t, err)
		require.Equal(t, "healthcare-provider-456", claims.Subject)
		require.Equal(t, "Dr. Jane Smith", claims.Name)
		require.Equal(t, []string{"Doctor", "Specialist"}, claims.Roles)

		// Create HTTP client with custom JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint with custom JWT token
		req, err := http.NewRequest("POST", cpcHttpService.URL+"/cpc/external/fhir/Patient/_search", strings.NewReader("identifier=1333333337"))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validToken)
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was successful
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the external endpoint was called
		require.True(t, externalEndpointCalled, "External endpoint should have been called")

		// Verify the request body was forwarded correctly
		require.Equal(t, "identifier=1333333337", receivedRequestBody)
	})

	t.Run("JWT token validation with signature verification", func(t *testing.T) {
		// Reset the external endpoint call tracker
		externalEndpointCalled = false

		// Create a valid JWT token
		validToken, err := tokenGen.CreateToken(map[string]interface{}{
			"sub":   "test-user-789",
			"name":  "Test User with Signature",
			"email": "signature-test@example.com",
		})
		require.NoError(t, err)

		// Validate the token with signature verification enabled
		claims, err := mockTokenClient.ValidateToken(ctx, validToken, rp.WithValidateSignature(true))
		require.NoError(t, err)
		require.Equal(t, "test-user-789", claims.Subject)
		require.Equal(t, "Test User with Signature", claims.Name)

		// Create HTTP client with valid JWT token
		client := &http.Client{}

		// Make request to CPC external endpoint
		req, err := http.NewRequest("GET", cpcHttpService.URL+"/cpc/external/fhir/Patient", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+validToken)
		req.Header.Set("X-Scp-Fhir-Url", mockExternalEndpoint.URL)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Verify the request was successful
		require.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify the external endpoint was called
		require.True(t, externalEndpointCalled, "External endpoint should have been called with valid signature")
	})
}

func setupIntegrationTest(t *testing.T, notificationEndpoint *url.URL) (*url.URL, *httptest.Server, *url.URL) {
	fhirBaseURL = test.SetupHAPI(t)
	config := careplanservice.DefaultConfig()
	config.Enabled = true

	cpsFHIRClient := fhirclient.New(fhirBaseURL, http.DefaultClient, nil)
	taskengine.LoadTestQuestionnairesAndHealthcareSevices(t, cpsFHIRClient)

	activeProfile := profile.TestProfile{
		Principal: auth.TestPrincipal1,
		CSD:       profile.TestCsdDirectory{Endpoint: notificationEndpoint.String()},
	}
	messageBroker, err := messaging.New(messaging.Config{}, nil)
	require.NoError(t, err)
	tenantCfg := tenants.Test(func(properties *tenants.Properties) {
		properties.CPSFHIR = coolfhir.ClientConfig{
			BaseURL: fhirBaseURL.String(),
		}
	})
	service, err := careplanservice.New(config, tenantCfg, activeProfile, orcaPublicURL.JoinPath("cps"), messageBroker, events.NewManager(messageBroker))
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService = httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")
	sessionManager, _ := createTestSession()

	// TODO: Tests using the Zorgplatform service
	cpsProxy := coolfhir.NewProxy("CPS->CPC", fhirBaseURL, "/cpc/fhir", orcaPublicURL, httpService.Client().Transport, true, false)

	cpcConfig := DefaultConfig()
	cpcConfig.Enabled = true
	cpcConfig.FHIR.BaseURL = fhirBaseURL.String()
	cpcConfig.HealthDataViewEndpointEnabled = true

	cpc, err := New(cpcConfig, tenants.Test(), profile.TestProfile{}, orcaPublicURL, sessionManager, messageBroker, events.NewManager(messageBroker), cpsProxy, cpsFHIRClient, carePlanServiceURL, nil)
	require.NoError(t, err)

	cpcServerMux := http.NewServeMux()
	cpcHttpService := httptest.NewServer(cpcServerMux)
	cpc.RegisterHandlers(cpcServerMux)
	cpcURL, _ := url.Parse(cpcHttpService.URL + "/cpc/fhir")

	// Return the URLs and httpService of the cpsDataRequester, cpcDataRequester, cpsDataHolder
	// these will be used to construct clients with both the correct and incorrect auth for positive and negative testing
	return carePlanServiceURL, httpService, cpcURL
}

func setupNotificationEndpoint(t *testing.T) *url.URL {
	notificationEndpoint := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		notificationCounter.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		notificationEndpoint.Close()
	})
	u, _ := url.Parse(notificationEndpoint.URL)
	return u
}
