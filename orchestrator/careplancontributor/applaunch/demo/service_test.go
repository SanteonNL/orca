package demo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/stretchr/testify/require"
)

func TestService_handle(t *testing.T) {
	t.Run("root base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager()
		service := Service{sessionManager: sessionManager, baseURL: "/", landingUrlPath: "/cpc/"}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/cpc/", response.Header().Get("Location"))
	})
	t.Run("subpath base URL", func(t *testing.T) {
		sessionManager := user.NewSessionManager()
		service := Service{sessionManager: sessionManager, baseURL: "/orca", landingUrlPath: "/cpc/"}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir", nil)

		service.handle(response, request)

		require.Equal(t, http.StatusFound, response.Code)
		require.Equal(t, "/orca/cpc/", response.Header().Get("Location"))
	})
	t.Run("should destroy previous session", func(t *testing.T) {
		sessionManager := user.NewSessionManager()
		service := Service{sessionManager: sessionManager, baseURL: "/orca", landingUrlPath: "/cpc/"}
		response := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir", nil)

		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())

		// Now launch the second session - copy the cookies so the session is retained
		cookies := response.Result().Cookies()
		response = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/demo-app-launch?patient=a&serviceRequest=b&practitioner=c&iss=https://example.com/fhir", nil)
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		service.handle(response, request)
		require.Equal(t, 1, sessionManager.SessionCount())

	})
}
