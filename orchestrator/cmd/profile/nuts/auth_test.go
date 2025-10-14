package nuts

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDutchNutsProfile_Authenticator(t *testing.T) {
	introspectionEndpoint := setupAuthorizationServer(t)
	profile := DutchNutsProfile{
		Config: Config{
			API: APIConfig{
				URL: introspectionEndpoint.String(),
			},
		},
		nutsAPIHTTPClient: http.DefaultClient,
	}
	t.Run("authenticated", func(t *testing.T) {
		var capturedPrincipal auth.Principal
		var capturedError error
		handler := profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
			slog.InfoContext(request.Context(), "test")
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
	t.Run("multi-tenancy", func(t *testing.T) {
		t.Run("token issuer doesn't match tenant", func(t *testing.T) {
			// access token was issued by http://localhost:8080/oauth2/test (subject=test),
			// but HTTP request is for tenant "other"
			var called bool
			tenantCfg := tenants.Config{
				"other": tenants.Properties{
					ID: "other",
					Nuts: tenants.NutsProperties{
						Subject: "other",
					},
				},
				"test": tenants.Test().Sole(),
			}
			handler := tenantCfg.HttpHandler(profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
				called = true
			}))
			httpRequest := httptest.NewRequest("GET", "/", nil)
			httpRequest.SetPathValue("tenant", "other")
			httpRequest.Header.Add("Authorization", "Bearer valid")
			httpResponse := httptest.NewRecorder()

			// Set up log capturing to check on the authorization failure reason
			oldDefaultLogger := slog.Default()
			defer slog.SetDefault(oldDefaultLogger)
			capturedLogs := new(bytes.Buffer)
			capturingLogger := slog.New(slog.NewJSONHandler(capturedLogs, nil))
			slog.SetDefault(capturingLogger)

			handler(httpResponse, httpRequest)

			require.False(t, called)
			require.Equal(t, http.StatusUnauthorized, httpResponse.Code)
			require.Contains(t, capturedLogs.String(), "Nuts access token issuer does not match tenant")
		})
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
				"scope":             "careplanservice",
				"iss":               "http://localhost:8080/oauth2/test",
				"client_id":         "http://localhost:8080/oauth2/other",
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
