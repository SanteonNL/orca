package nuts

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDutchNutsProfile_Authenticator(t *testing.T) {
	introspectionEndpoint := setupAuthorizationServer(t)
	t.Run("authenticated", func(t *testing.T) {
		profile := DutchNutsProfile{
			Config: Config{
				API: APIConfig{
					URL: introspectionEndpoint.String(),
				},
			},
			nutsAPIHTTPClient: http.DefaultClient,
		}

		var capturedPrincipal auth.Principal
		var capturedError error
		handler := profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
			log.Ctx(request.Context()).Info().Msg("test")
			capturedPrincipal, capturedError = auth.PrincipalFromContext(request.Context())
			_, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusOK)
		})
		httpRequest := httptest.NewRequest("GET", "/", nil)
		httpRequest.Header.Add("Authorization", "Bearer valid")

		handler(httptest.NewRecorder(), httpRequest)

		require.NoError(t, capturedError)
		require.Equal(t, *capturedPrincipal.Organization.Name, "Hospital")
		require.Len(t, capturedPrincipal.Organization.Identifier, 1)
		require.Equal(t, *capturedPrincipal.Organization.Identifier[0].System, coolfhir.URANamingSystem)
		require.Equal(t, *capturedPrincipal.Organization.Identifier[0].Value, "1")
		require.Len(t, capturedPrincipal.Organization.Address, 1)
		require.Equal(t, *capturedPrincipal.Organization.Address[0].City, "CareTown")
	})
	t.Run("invalid token", func(t *testing.T) {
		profile := DutchNutsProfile{
			Config: Config{
				API: APIConfig{
					URL: introspectionEndpoint.String(),
				},
			},
			nutsAPIHTTPClient: http.DefaultClient,
		}

		var capturedError error
		handler := profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
			assert.Fail(t, "Should not reach here")
		})
		httpRequest := httptest.NewRequest("GET", "/", nil)
		httpRequest.Header.Add("Authorization", "Bearer invalid")
		httpResponse := httptest.NewRecorder()

		handler(httpResponse, httpRequest)

		require.NoError(t, capturedError)
		require.Equal(t, http.StatusUnauthorized, httpResponse.Code)
	})
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
