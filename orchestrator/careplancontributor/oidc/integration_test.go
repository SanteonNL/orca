package oidc

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	zitadelHTTP "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

func TestService_IntegrationTest(t *testing.T) {
	sessionData := user.SessionData{
		// TODO
	}
	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	issuerURL := must.ParseURL(httpServer.URL + "/provider")
	clientURL := must.ParseURL(httpServer.URL + "/client")
	clientRedirectURL := clientURL.JoinPath("callback")
	clientLoginURL := clientURL.JoinPath("login")
	requestedScopes := []string{"openid", "profile", "email"}
	const clientID = "test-client-id"
	const clientSecret = ""

	// Setup OIDC provider
	service, err := New(false, issuerURL, Config{
		Enabled: true,
		Clients: map[string]ClientConfig{
			"test": {
				ID:          clientID,
				RedirectURI: clientRedirectURL.String(),
			},
		},
	})
	require.NoError(t, err)
	mux.Handle(issuerURL.Path+"/", http.StripPrefix(issuerURL.Path, service))
	mux.HandleFunc(issuerURL.JoinPath("login").Path, func(writer http.ResponseWriter, request *http.Request) {
		service.HandleLogin(writer, request, &sessionData)
	})

	// Setup OIDC client
	ctx := context.Background()
	clientKey := make([]byte, 32)
	clientCookieHandler := zitadelHTTP.NewCookieHandler(clientKey, clientKey, zitadelHTTP.WithUnsecure())
	clientOpts := []rp.Option{
		rp.WithCookieHandler(clientCookieHandler),
		rp.WithPKCE(clientCookieHandler),
	}
	client, err := rp.NewRelyingPartyOIDC(ctx, issuerURL.String(), clientID, clientSecret, clientRedirectURL.String(), requestedScopes, clientOpts...)
	require.NoError(t, err)
	mux.Handle(clientLoginURL.Path, rp.AuthURLHandler(func() string {
		return "fixed-state"
	}, client))
	var capturedIDTokenClaims *oidc.IDTokenClaims
	mux.Handle(clientRedirectURL.Path, rp.CodeExchangeHandler(rp.UserinfoCallback(func(w http.ResponseWriter, r *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], state string, provider rp.RelyingParty, info *oidc.UserInfo) {
		capturedIDTokenClaims = tokens.IDTokenClaims
	}), client))

	// Perform test
	clientCookieJar, err := cookiejar.New(nil)
	require.NoError(t, err)
	httpClient := http.Client{
		Jar: clientCookieJar,
	}
	_, err = httpClient.Get(clientLoginURL.String())
	require.NoError(t, err)

	assert.Equal(t, "12345", capturedIDTokenClaims.Subject)
	assert.Equal(t, "John Doe", capturedIDTokenClaims.GetUserInfo().Name)
	assert.Equal(t, "john@example.com", capturedIDTokenClaims.GetUserInfo().Email)
	assert.Equal(t, []any{"Verpleegkundige niveau 4"}, capturedIDTokenClaims.Claims["roles"])

}
