package rp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealisticTokens(t *testing.T) {
	// Create a token generator with realistic Azure AD B2C claims
	tokenGen, err := NewTestTokenGenerator()
	require.NoError(t, err)
	require.NotNil(t, tokenGen)

	ctx := context.Background()
	client, err := NewMockClient(ctx, tokenGen)
	require.NoError(t, err)
	require.NotNil(t, client)

	t.Run("Valid token with standard claims, ok", func(t *testing.T) {
		// Create a token with default claims
		token, err := tokenGen.CreateToken(nil)
		require.NoError(t, err)

		claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
		require.NoError(t, err)
		require.NotNil(t, claims)

		// Verify some standard claims
		assert.Equal(t, "12345678-1234-1234-1234-123456789012", claims.Subject)
		assert.Equal(t, tokenGen.ClientID, claims.Audience[0])
		assert.Equal(t, []string{"test.user@example.com"}, claims.Emails)
	})

	t.Run("Token with custom roles, ok", func(t *testing.T) {
		roles := []string{"Admin", "User", "Approver"}
		token, err := tokenGen.CreateTokenWithRoles(roles)
		require.NoError(t, err)

		claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
		require.NoError(t, err)
		require.NotNil(t, claims)

		// Verify the roles
		assert.Equal(t, roles, claims.Roles)
	})

	t.Run("Expired token, fails", func(t *testing.T) {
		token, err := tokenGen.CreateExpiredToken()
		require.NoError(t, err)

		claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
		assert.Contains(t, err.Error(), "exp")
	})

	t.Run("Invalid issuer, fails", func(t *testing.T) {
		token, err := tokenGen.CreateInvalidIssuerToken()
		require.NoError(t, err)

		claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "untrusted token issuer")
	})

	t.Run("Invalid audience, fails", func(t *testing.T) {
		token, err := tokenGen.CreateInvalidAudienceToken()
		require.NoError(t, err)

		claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
		assert.Contains(t, err.Error(), "aud")
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
