package verification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func testService(t *testing.T) (*Service, *rp.TestTokenGenerator, *user.SessionManager[session.Data], tenants.Properties) {
	t.Helper()

	generator, err := rp.NewTestTokenGenerator()
	require.NoError(t, err)
	validator, err := rp.NewMockClient(context.Background(), generator)
	require.NoError(t, err)

	tenantCfg := tenants.Test()
	tenant := tenantCfg.Sole()

	globals.RegisterCPSFHIRClient(tenant.ID, &test.StubFHIRClient{
		Resources: []interface{}{
			fhir.Task{
				Id:         to.Ptr("task-1"),
				Status:     fhir.TaskStatusInProgress,
				Intent:     "order",
				For:        &fhir.Reference{Reference: to.Ptr("Patient/patient-1")},
				Focus:      &fhir.Reference{Reference: to.Ptr("ServiceRequest/sr-1")},
				Identifier: []fhir.Identifier{{System: to.Ptr("unit-test-system"), Value: to.Ptr("10")}},
				ReasonCode: &fhir.CodeableConcept{
					Coding: []fhir.Coding{{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("13645005")}},
				},
			},
			fhir.Patient{Id: to.Ptr("patient-1")},
			fhir.ServiceRequest{Id: to.Ptr("sr-1"), Status: fhir.RequestStatusActive, Intent: fhir.RequestIntentOrder},
			fhir.Task{
				Id:     to.Ptr("task-no-patient"),
				Status: fhir.TaskStatusInProgress,
				Intent: "order",
			},
		},
	})

	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	service := &Service{
		sessionManager:     sessionManager,
		config:             Config{Enabled: true},
		tenants:            tenantCfg,
		orcaPublicURL:      must.ParseURL("https://orca.example.com/orca"),
		frontendLandingUrl: must.ParseURL("/frontend/enrollment"),
		profile:            profile.TestProfile{Principal: auth.TestPrincipal1},
		tokenValidator:     validator,
		nonces:             newNonceStore(launchURLTTL),
	}
	return service, generator, sessionManager, tenant
}

func createLaunch(t *testing.T, service *Service, token string, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest("POST", "/verification-launch", strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	service.handleCreate(response, request)
	return response
}

func TestService_VerificationLaunch(t *testing.T) {
	t.Run("full flow: create, redeem once, session mirrors a real launch", func(t *testing.T) {
		service, generator, sessionManager, tenant := testService(t)
		token, err := generator.CreateToken(map[string]interface{}{"name": "Wiebe (verification)"})
		require.NoError(t, err)

		response := createLaunch(t, service, token, `{"tenant":"`+tenant.ID+`","taskId":"task-1","practitionerName":"Dr. Verification"}`)
		require.Equal(t, http.StatusOK, response.Code)

		var launch createLaunchResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &launch))
		require.Contains(t, launch.LaunchURL, "https://orca.example.com/orca/verification-launch/")

		nonce := launch.LaunchURL[strings.LastIndex(launch.LaunchURL, "/")+1:]
		redeemRequest := httptest.NewRequest("GET", "/verification-launch/"+nonce, nil)
		redeemRequest.SetPathValue("nonce", nonce)
		redeemResponse := httptest.NewRecorder()
		service.handleRedeem(redeemResponse, redeemRequest)

		require.Equal(t, http.StatusFound, redeemResponse.Code)
		assert.Equal(t, "/frontend/enrollment/task/task-1", redeemResponse.Header().Get("Location"))

		sessionData := user.SessionFromHttpResponse(sessionManager, redeemResponse.Result())
		require.NotNil(t, sessionData)
		assert.Equal(t, fhirLauncherKey, sessionData.FHIRLauncher)
		assert.Equal(t, tenant.ID, sessionData.TenantID)
		assert.Equal(t, "Patient/patient-1", sessionData.GetByType("Patient").Path)
		assert.Equal(t, "ServiceRequest/sr-1", sessionData.GetByType("ServiceRequest").Path)
		assert.Equal(t, "unit-test-system|10", *sessionData.TaskIdentifier)

		practitioner := session.Get[fhir.Practitioner](sessionData)
		require.NotNil(t, practitioner)
		assert.Equal(t, "Dr. Verification", *practitioner.Name[0].Text)

		condition := session.Get[fhir.Condition](sessionData)
		require.NotNil(t, condition)
		assert.Equal(t, "13645005", *condition.Code.Coding[0].Code)

		require.NotNil(t, session.Get[fhir.Organization](sessionData))

		// single use
		secondRedeem := httptest.NewRecorder()
		retryRequest := httptest.NewRequest("GET", "/verification-launch/"+nonce, nil)
		retryRequest.SetPathValue("nonce", nonce)
		service.handleRedeem(secondRedeem, retryRequest)
		assert.Equal(t, http.StatusNotFound, secondRedeem.Code)
	})

	t.Run("practitioner falls back to token claims when not asserted", func(t *testing.T) {
		service, generator, sessionManager, tenant := testService(t)
		token, err := generator.CreateToken(map[string]interface{}{"name": "Wiebe (verification)"})
		require.NoError(t, err)

		response := createLaunch(t, service, token, `{"tenant":"`+tenant.ID+`","taskId":"task-1"}`)
		require.Equal(t, http.StatusOK, response.Code)

		var launch createLaunchResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &launch))
		nonce := launch.LaunchURL[strings.LastIndex(launch.LaunchURL, "/")+1:]
		redeemRequest := httptest.NewRequest("GET", "/verification-launch/"+nonce, nil)
		redeemRequest.SetPathValue("nonce", nonce)
		redeemResponse := httptest.NewRecorder()
		service.handleRedeem(redeemResponse, redeemRequest)

		sessionData := user.SessionFromHttpResponse(sessionManager, redeemResponse.Result())
		require.NotNil(t, sessionData)
		practitioner := session.Get[fhir.Practitioner](sessionData)
		require.NotNil(t, practitioner)
		assert.Equal(t, "Wiebe (verification)", *practitioner.Name[0].Text)
	})

	t.Run("missing token is rejected", func(t *testing.T) {
		service, _, _, tenant := testService(t)
		response := createLaunch(t, service, "", `{"tenant":"`+tenant.ID+`","taskId":"task-1"}`)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("token for another audience is rejected", func(t *testing.T) {
		service, generator, _, tenant := testService(t)
		token, err := generator.CreateInvalidAudienceToken()
		require.NoError(t, err)
		response := createLaunch(t, service, token, `{"tenant":"`+tenant.ID+`","taskId":"task-1"}`)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		service, generator, _, tenant := testService(t)
		token, err := generator.CreateExpiredToken()
		require.NoError(t, err)
		response := createLaunch(t, service, token, `{"tenant":"`+tenant.ID+`","taskId":"task-1"}`)
		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})

	t.Run("unknown tenant is rejected", func(t *testing.T) {
		service, generator, _, _ := testService(t)
		token, err := generator.CreateToken(nil)
		require.NoError(t, err)
		response := createLaunch(t, service, token, `{"tenant":"nope","taskId":"task-1"}`)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("task without a patient is rejected", func(t *testing.T) {
		service, generator, _, tenant := testService(t)
		token, err := generator.CreateToken(nil)
		require.NoError(t, err)
		response := createLaunch(t, service, token, `{"tenant":"`+tenant.ID+`","taskId":"task-no-patient"}`)
		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	})

	t.Run("unknown nonce is rejected", func(t *testing.T) {
		service, _, _, _ := testService(t)
		request := httptest.NewRequest("GET", "/verification-launch/nope", nil)
		request.SetPathValue("nonce", "nope")
		response := httptest.NewRecorder()
		service.handleRedeem(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

// nodeCallingTestProfile hands out a client that trusts the test node's TLS certificate.
type nodeCallingTestProfile struct {
	profile.TestProfile
	client *http.Client
}

func (p nodeCallingTestProfile) HttpClient(context.Context, fhir.Identifier) (*http.Client, error) {
	return p.client, nil
}

func TestService_DelegatesToNodeHoldingTheTask(t *testing.T) {
	t.Run("relays the launch URL of the node that owns the Task", func(t *testing.T) {
		var receivedPath, receivedBody string
		remoteNode := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			receivedPath = request.URL.Path
			body, _ := io.ReadAll(request.Body)
			receivedBody = string(body)
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"launchUrl":"https://hospital.example.com/orca/verification-launch/abc","expiresInSeconds":60}`))
		}))
		defer remoteNode.Close()

		service, generator, _, _ := testService(t)
		service.profile = nodeCallingTestProfile{TestProfile: profile.TestProfile{Principal: auth.TestPrincipal1}, client: remoteNode.Client()}
		token, err := generator.CreateToken(nil)
		require.NoError(t, err)

		taskRef := remoteNode.URL + "/orca/cps/saz/Task/task-42"
		response := createLaunch(t, service, token, `{"taskReference":"`+taskRef+`","practitionerName":"Wiebe"}`)

		require.Equal(t, http.StatusOK, response.Code)
		var launch createLaunchResponse
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &launch))
		// the URL must stay on the owning node, so its frontend and OIDC provider serve the launch
		assert.Equal(t, "https://hospital.example.com/orca/verification-launch/abc", launch.LaunchURL)
		assert.Equal(t, "/orca/verification-launch/scp", receivedPath)
		assert.Contains(t, receivedBody, taskRef)
		assert.Contains(t, receivedBody, "Wiebe")
	})

	t.Run("a delegated launch is not delegated onwards", func(t *testing.T) {
		service, _, _, _ := testService(t)
		request := httptest.NewRequest("POST", "/verification-launch/scp",
			strings.NewReader(`{"taskReference":"https://elsewhere.example.com/orca/cps/saz/Task/task-1"}`))
		request = request.WithContext(auth.WithPrincipal(request.Context(), *auth.TestPrincipal1))
		response := httptest.NewRecorder()

		service.handleCreateForNode(response, request)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("unauthenticated node request is rejected", func(t *testing.T) {
		service, _, _, _ := testService(t)
		request := httptest.NewRequest("POST", "/verification-launch/scp", strings.NewReader(`{}`))
		response := httptest.NewRecorder()

		service.handleCreateForNode(response, request)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})
}

func TestService_WithoutOIDC_OnlyServesDelegatedLaunches(t *testing.T) {
	// A node that no workforce application calls directly must not expose the token-authenticated
	// entrypoint at all, so no identity provider of another party has to be trusted there.
	service, err := New(
		context.Background(),
		Config{Enabled: true},
		user.NewSessionManager[session.Data](time.Minute),
		tenants.Test(),
		must.ParseURL("https://orca.example.com/orca"),
		must.ParseURL("/frontend/enrollment"),
		profile.TestProfile{Principal: auth.TestPrincipal1},
	)
	require.NoError(t, err)

	mux := http.NewServeMux()
	service.RegisterHandlers(mux)

	_, entraPattern := mux.Handler(httptest.NewRequest("POST", "/verification-launch", nil))
	assert.Empty(t, entraPattern, "the OIDC entrypoint must not be registered without OIDC configuration")
	_, scpPattern := mux.Handler(httptest.NewRequest("POST", "/verification-launch/scp", nil))
	assert.Equal(t, "POST /verification-launch/scp", scpPattern)

	proxies, clients := service.CreateEHRProxies()
	assert.Empty(t, proxies)
	assert.Empty(t, clients)
}

func TestService_LaunchRequestErrors(t *testing.T) {
	token := func(t *testing.T, g *rp.TestTokenGenerator) string {
		t.Helper()
		tok, err := g.CreateToken(nil)
		require.NoError(t, err)
		return tok
	}

	t.Run("malformed body is rejected", func(t *testing.T) {
		service, generator, _, _ := testService(t)
		response := createLaunch(t, service, token(t, generator), `not json`)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("body without a Task is rejected", func(t *testing.T) {
		service, generator, _, _ := testService(t)
		response := createLaunch(t, service, token(t, generator), `{}`)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("delegating for an unknown tenant is rejected", func(t *testing.T) {
		service, generator, _, _ := testService(t)
		response := createLaunch(t, service, token(t, generator),
			`{"tenant":"nope","taskReference":"https://hospital.example.com/orca/cps/saz/Task/task-1"}`)
		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"remote node rejects the launch", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }},
		{"remote node returns an unreadable launch", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{`)) }},
		{"remote node returns no launch URL", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }},
	} {
		t.Run(tc.name+" surfaces as bad gateway", func(t *testing.T) {
			remoteNode := httptest.NewTLSServer(tc.handler)
			defer remoteNode.Close()

			service, generator, _, _ := testService(t)
			service.profile = nodeCallingTestProfile{TestProfile: profile.TestProfile{Principal: auth.TestPrincipal1}, client: remoteNode.Client()}

			response := createLaunch(t, service, token(t, generator),
				`{"taskReference":"`+remoteNode.URL+`/orca/cps/saz/Task/task-1"}`)
			assert.Equal(t, http.StatusBadGateway, response.Code)
		})
	}
}

func TestSplitTaskReference(t *testing.T) {
	t.Run("splits CPS base URL and Task ID", func(t *testing.T) {
		base, taskID, err := splitTaskReference("https://hospital.example.com/orca/cps/saz/Task/abc")
		require.NoError(t, err)
		assert.Equal(t, "https://hospital.example.com/orca/cps/saz", base.String())
		assert.Equal(t, "abc", taskID)
		assert.Equal(t, "https://hospital.example.com/orca", nodeBaseURL(base).String())
	})
	for _, invalid := range []string{
		"http://hospital.example.com/orca/cps/saz/Task/abc",
		"/orca/cps/saz/Task/abc",
		"https://hospital.example.com/orca/cps/saz/Task/abc/_history/1",
		"https://hospital.example.com/orca/cps/saz/CarePlan/abc",
	} {
		t.Run("rejects "+invalid, func(t *testing.T) {
			_, _, err := splitTaskReference(invalid)
			assert.Error(t, err)
		})
	}
}

func TestNonceStore(t *testing.T) {
	t.Run("expired nonce cannot be redeemed", func(t *testing.T) {
		store := newNonceStore(time.Millisecond)
		nonce := store.Put(session.Data{TenantID: "test"})
		time.Sleep(5 * time.Millisecond)

		_, ok := store.Take(nonce)
		assert.False(t, ok)
	})

	t.Run("nonce is single use", func(t *testing.T) {
		store := newNonceStore(time.Minute)
		nonce := store.Put(session.Data{TenantID: "test"})

		_, first := store.Take(nonce)
		_, second := store.Take(nonce)
		assert.True(t, first)
		assert.False(t, second)
	})
}
