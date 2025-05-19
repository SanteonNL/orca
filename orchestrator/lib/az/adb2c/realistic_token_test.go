package adb2c

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealisticTokens(t *testing.T) {
	// Create a token generator with realistic Azure AD B2C claims
	tokenGen, err := NewTestTokenGenerator()
	require.NoError(t, err)
	require.NotNil(t, tokenGen)

	jwksJSON, err := tokenGen.GetJWKSetJSON()
	require.NoError(t, err)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksJSON)
	}))
	defer jwkServer.Close()

	openIDConfig := OpenIDConfig{
		Issuer:                tokenGen.GetIssuerURL(),
		JwksURI:               jwkServer.URL,
		AuthorizationEndpoint: "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/authorize",
		TokenEndpoint:         "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/token",
		EndSessionEndpoint:    "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/logout",
	}

	openIDConfigJSON, err := json.Marshal(openIDConfig)
	require.NoError(t, err)

	openIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(openIDConfigJSON)
	}))
	defer openIDServer.Close()

	issuerURL := tokenGen.GetIssuerURL()
	trustedIssuers := map[string]string{
		issuerURL: openIDServer.URL,
	}

	// Create a client that uses our mock servers
	ctx := context.Background()
	client, err := NewClient(ctx, trustedIssuers, tokenGen.ClientID, WithDefaultIssuer(issuerURL))
	require.NoError(t, err)

	t.Run("Valid token with standard claims", func(t *testing.T) {
		// Create a token with default claims
		token, err := tokenGen.CreateToken(nil)
		require.NoError(t, err)

		// Validate the token
		ctx := context.Background()
		claims, err := client.ValidateToken(ctx, token)
		require.NoError(t, err)
		require.NotNil(t, claims)

		// Verify some standard claims
		assert.Equal(t, "12345678-1234-1234-1234-123456789012", claims.Subject)
		assert.Equal(t, tokenGen.ClientID, claims.Audience[0])
		assert.Equal(t, []string{"test.user@example.com"}, claims.Emails)
	})

	t.Run("Token with custom roles", func(t *testing.T) {
		roles := []string{"Admin", "User", "Approver"}
		token, err := tokenGen.CreateTokenWithRoles(roles)
		require.NoError(t, err)

		// Validate the token
		ctx := context.Background()
		claims, err := client.ValidateToken(ctx, token)
		require.NoError(t, err)
		require.NotNil(t, claims)

		// Verify the roles
		assert.Equal(t, roles, claims.Roles)
	})

	t.Run("Expired token", func(t *testing.T) {
		token, err := tokenGen.CreateExpiredToken()
		require.NoError(t, err)

		// Validate the token - should fail
		ctx := context.Background()
		claims, err := client.ValidateToken(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token has expired")
	})

	t.Run("Invalid issuer", func(t *testing.T) {
		token, err := tokenGen.CreateInvalidIssuerToken()
		require.NoError(t, err)

		// Validate the token - should fail
		ctx := context.Background()
		claims, err := client.ValidateToken(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "untrusted token issuer")
	})

	t.Run("Invalid audience", func(t *testing.T) {
		token, err := tokenGen.CreateInvalidAudienceToken()
		require.NoError(t, err)

		// Validate the token - should fail
		ctx := context.Background()
		claims, err := client.ValidateToken(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "invalid token audience")
	})

	t.Run("Parse token and inspect claims", func(t *testing.T) {
		customClaims := map[string]interface{}{
			"extension_CustomAttribute": "custom-value",
			"groups": []string{
				"group1",
				"group2",
			},
		}

		token, err := tokenGen.CreateToken(customClaims)
		require.NoError(t, err)

		// Parse the token to inspect its claims
		claims, err := ParseToken(token)
		require.NoError(t, err)

		// Verify standard claims
		assert.Equal(t, "12345678-1234-1234-1234-123456789012", claims["sub"])
		assert.Equal(t, "Test User", claims["name"])
		assert.Equal(t, "test.user@example.com", claims["preferred_username"])

		// Verify emails is an array
		emails, ok := claims["emails"].([]interface{})
		require.True(t, ok, "emails should be an array")
		assert.Equal(t, "test.user@example.com", emails[0])

		// Verify custom claims
		assert.Equal(t, "custom-value", claims["extension_CustomAttribute"])
		groups, ok := claims["groups"].([]interface{})
		require.True(t, ok)
		assert.Len(t, groups, 2)
		assert.Equal(t, "group1", groups[0])
		assert.Equal(t, "group2", groups[1])

		// Print the token structure for inspection
		prettyJSON, err := json.MarshalIndent(claims, "", "  ")
		require.NoError(t, err)
		t.Logf("Token claims: \n%s", string(prettyJSON))
	})
}
