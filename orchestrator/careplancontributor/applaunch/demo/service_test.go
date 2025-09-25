package demo

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/stretchr/testify/assert"

	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestService_handle(t *testing.T) {
	existingTask := fhir.Task{
		Id: to.Ptr("12345678910"),
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("unit-test-system"),
				Value:  to.Ptr("20"),
			},
		},
	}
	ehrFHIRClient := &test.StubFHIRClient{
		Resources: []interface{}{
			fhir.Practitioner{
				Id: to.Ptr("c"),
			},
			fhir.Patient{
				Id: to.Ptr("a"),
			},
		},
	}

	tenantCfg := tenants.Test(func(properties *tenants.Properties) {
		properties.Demo = tenants.DemoProperties{
			FHIR: coolfhir.ClientConfig{
				BaseURL: "https://example.com/fhir",
			},
		}
	})
	tenant := tenantCfg.Sole()
	globals.RegisterCPSFHIRClient(tenant.ID, &test.StubFHIRClient{
		Resources: []interface{}{
			existingTask,
		},
	})

	t.Run("root base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager[session.Data](time.Minute)
		service := Service{
			sessionManager: sessionManager, orcaPublicURL: must.ParseURL("/"),
			frontendLandingUrl: must.ParseURL("/cpc/"),
			ehrFHIRClientFactory: func(_ *url.URL, _ *http.Client) fhirclient.Client {
				return ehrFHIRClient
			},
			profile: profile.TestProfile{Principal: auth.TestPrincipal1},
			tenants: tenantCfg,
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=Patient/a&serviceRequest=ServiceRequest/b&practitioner=Practitioner/c&tenant="+tenant.ID+"&taskIdentifier=unit-test-system|10", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/cpc/new", response.Header().Get("Location"))
		sessionData := user.SessionFromHttpResponse(sessionManager, response.Result())
		require.NotNil(t, sessionData)
		require.Equal(t, "Patient/a", sessionData.GetByType("Patient").Path)
		require.Equal(t, "ServiceRequest/b", sessionData.GetByType("ServiceRequest").Path)
		require.Equal(t, "Practitioner/c", sessionData.GetByType("Practitioner").Path)
		require.Equal(t, "unit-test-system|10", *sessionData.TaskIdentifier)
	})
	t.Run("path traversal is disallowed", func(t *testing.T) {
		sessionManager := user.NewSessionManager[session.Data](time.Minute)
		service := Service{
			sessionManager: sessionManager, orcaPublicURL: must.ParseURL("/"),
			frontendLandingUrl: must.ParseURL("/cpc/"),
			ehrFHIRClientFactory: func(fhirBaseURL *url.URL, _ *http.Client) fhirclient.Client {
				return fhirclient.New(fhirBaseURL, nil, nil)
			},
			profile: profile.TestProfile{Principal: auth.TestPrincipal1},
			tenants: tenantCfg,
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=../Patient/a&serviceRequest=../ServiceRequest/b&practitioner=../Practitioner/c&tenant="+tenant.ID+"&taskIdentifier=unit-test-system|10", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "FHIR request URL is outside the base URL hierarchy")
	})
	t.Run("subpath base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager[session.Data](time.Minute)
		service := Service{
			sessionManager:     sessionManager,
			orcaPublicURL:      must.ParseURL("/orca"),
			frontendLandingUrl: must.ParseURL("/frontend/landing"),
			ehrFHIRClientFactory: func(_ *url.URL, _ *http.Client) fhirclient.Client {
				return ehrFHIRClient
			},
			profile: profile.TestProfile{Principal: auth.TestPrincipal1},
			tenants: tenantCfg,
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=Patient/a&serviceRequest=b&practitioner=Practitioner/c&tenant="+tenant.ID+"&taskIdentifier=unit-test-system|10", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/frontend/landing/new", response.Header().Get("Location"))
	})
	t.Run("should destroy previous session", func(t *testing.T) {
		sessionManager := user.NewSessionManager[session.Data](time.Minute)
		service := Service{
			sessionManager:     sessionManager,
			orcaPublicURL:      must.ParseURL("/orca"),
			frontendLandingUrl: must.ParseURL("/cpc/"),
			ehrFHIRClientFactory: func(_ *url.URL, _ *http.Client) fhirclient.Client {
				return ehrFHIRClient
			},
			profile: profile.TestProfile{Principal: auth.TestPrincipal1},
			tenants: tenantCfg,
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=Patient/a&serviceRequest=b&practitioner=Practitioner/c&tenant="+tenant.ID+"&taskIdentifier=unit-test-system|20", nil)

		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())

		// Now launch the second session - copy the cookies so the session is retained
		cookies := response.Result().Cookies()
		response = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/demo-app-launch?patient=Patient/a&serviceRequest=b&practitioner=Practitioner/c&iss=https://example.com/fhir&taskIdentifier=unit-test-system|20", nil)
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())
	})
	t.Run("should restore task", func(t *testing.T) {
		sessionManager := user.NewSessionManager[session.Data](time.Minute)
		service := Service{
			sessionManager:     sessionManager,
			orcaPublicURL:      must.ParseURL("/orca"),
			frontendLandingUrl: must.ParseURL("/frontend/enrollment"),
			ehrFHIRClientFactory: func(_ *url.URL, _ *http.Client) fhirclient.Client {
				return ehrFHIRClient
			},
			profile: profile.TestProfile{Principal: auth.TestPrincipal1},
			tenants: tenantCfg,
		}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=Patient/a&serviceRequest=b&practitioner=Practitioner/c&tenant="+tenant.ID+"&taskIdentifier=unit-test-system|20", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/frontend/enrollment/task/12345678910", response.Header().Get("Location"))
	})
}
