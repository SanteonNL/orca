package demo

import (
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
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
}
