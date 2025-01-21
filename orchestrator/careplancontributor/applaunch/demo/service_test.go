package demo

import (
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
				System: to.Ptr(demoTaskSystemSystem),
				Value:  to.Ptr("20"),
			},
		},
	}
	globals.CarePlanServiceFhirClient = &test.StubFHIRClient{
		Resources: []interface{}{
			existingTask,
		},
	}

	t.Run("root base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager(time.Minute)
		service := Service{sessionManager: sessionManager, baseURL: "/", frontendLandingUrl: must.ParseURL("/cpc/")}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir&taskIdentifier=10", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/cpc/new", response.Header().Get("Location"))
	})
	t.Run("subpath base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager(time.Minute)
		service := Service{sessionManager: sessionManager, baseURL: "/orca", frontendLandingUrl: must.ParseURL("/frontend/landing")}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir&taskIdentifier=10", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/frontend/landing/new", response.Header().Get("Location"))
	})
	t.Run("should destroy previous session", func(t *testing.T) {
		sessionManager := user.NewSessionManager(time.Minute)
		service := Service{sessionManager: sessionManager, baseURL: "/orca", frontendLandingUrl: must.ParseURL("/cpc/")}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir&taskIdentifier=10", nil)

		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())

		// Now launch the second session - copy the cookies so the session is retained
		cookies := response.Result().Cookies()
		response = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir&taskIdentifier=10", nil)
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())
	})
	t.Run("should restore task", func(t *testing.T) {
		sessionManager := user.NewSessionManager(time.Minute)
		service := Service{sessionManager: sessionManager, baseURL: "/orca", frontendLandingUrl: must.ParseURL("/frontend/enrollment")}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir&taskIdentifier=20", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/frontend/enrollment/task/12345678910", response.Header().Get("Location"))
	})
}
