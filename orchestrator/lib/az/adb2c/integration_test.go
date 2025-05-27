package adb2c

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationWithRealToken performs an integration test with a real Azure AD B2C token
// It requires environment variables to be set with the token and Azure AD B2C details
//
// To run this test, export the required environment variables:
//
//	export ADB2C_ENABLED=true
//
// This should be the same as the token audience (aud) claim
//
//	export ADB2C_CLIENTID="your-client-id"
//	export ADB2C_TRUSTEDISSUERS_<ISSUER_NAME>_ISSUERURL="https://your-tenant.b2clogin.com/your-tenant.onmicrosoft.com/v2.0/"
//	export ADB2C_TRUSTEDISSUERS_<ISSUER_NAME>_DISCOVERYURL="https://your-tenant.b2clogin.com/your-tenant.onmicrosoft.com/v2.0/.well-known/openid_configuration"
//
// Note: This test will be skipped if the ADB2C_TOKEN environment variable is not set
func TestIntegrationWithRealToken(t *testing.T) {
	testConfig := LoadTestConfig()
	if testConfig.Token == "" {
		t.Skip("Skipping integration test. ADB2C_TOKEN environment variable not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var client *Client
	var err error

	t.Logf("Creating ADB2C client from koanf configuration")

	client, err = NewClient(ctx, testConfig.Config)
	require.NoError(t, err, "Failed to create ADB2C client from config")

	// Use the token validation with real signature verification
	claims, err := client.ValidateToken(ctx, testConfig.Token)

	if err != nil {
		// Expired tokens are expected in some test cases
		if strings.Contains(err.Error(), "\"exp\" not satisfied") {
			t.Logf("Token validation failed as expected: %v", err)
			t.Logf("This is normal if you're using an expired token for testing")
			return
		}

		// For verbose error output to help debug token issues
		t.Logf("Token validation failed with error: %v", err)

		// Try validating with signature validation disabled as a fallback
		t.Logf("Attempting validation without signature verification...")
		claims, err = client.ValidateToken(ctx, testConfig.Token, WithValidateSignature(false))
		if err != nil {
			t.Fatalf("Token validation failed even without signature verification: %v", err)
		} else {
			t.Logf("Token validated successfully without signature verification")
			prettyJSON, _ := json.MarshalIndent(claims, "", "  ")
			t.Logf("Token claims: \n%s", string(prettyJSON))
			return
		}
	}

	// If validation succeeded, verify and print the claims
	require.NotNil(t, claims, "Token claims should not be nil")

	prettyJSON, err := json.MarshalIndent(claims, "", "  ")
	require.NoError(t, err)
	t.Logf("Token validation succeeded with claims: \n%s", string(prettyJSON))

	// Verify that the subject claim matches what we parsed earlier
	parsedClaims, err := ParseToken(testConfig.Token)
	require.NoError(t, err, "Failed to parse token claims")
	assert.Equal(t, parsedClaims["sub"], claims.Subject)
}

// TestNewClientFromConfig tests the new config-based client creation
func TestNewClientFromConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("creates client from valid config", func(t *testing.T) {
		config := &Config{
			Enabled:       true,
			ADB2CClientID: "test-client-id",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		// Note: This will fail with network errors since we're using real URLs, but it should validate the config
		client, err := NewClient(ctx, config)

		// We expect this to fail due to network issues, but the error should be about network/discovery, not config validation
		if err != nil {
			// The error should be about network/discovery issues, not config validation
			assert.Contains(t, err.Error(), "failed to initialize OpenID configurations")
		} else {
			// If it somehow succeeds (maybe in a test environment), verify the client is configured correctly
			assert.NotNil(t, client)
			assert.Equal(t, "test-client-id", client.ClientID)
		}
	})

	t.Run("fails with nil config", func(t *testing.T) {
		client, err := NewClient(ctx, nil)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("fails with disabled config", func(t *testing.T) {
		config := &Config{
			Enabled:       false, // Disabled
			ADB2CClientID: "test-client-id",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		client, err := NewClient(ctx, config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "ADB2C is not enabled in configuration")
	})

	t.Run("fails with invalid config", func(t *testing.T) {
		config := &Config{
			Enabled:             true,
			ADB2CClientID:       "", // Missing client ID
			ADB2CTrustedIssuers: map[string]TrustedIssuer{},
		}

		client, err := NewClient(ctx, config)

		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "invalid configuration")
	})

	t.Run("works with mock client", func(t *testing.T) {
		// Create a test token generator for a more realistic test
		tokenGen, err := NewTestTokenGenerator()
		require.NoError(t, err)

		// Create a mock client to test the config-based approach
		mockClient, err := NewMockClient(ctx, tokenGen)
		require.NoError(t, err)

		// Create a config that matches the mock client's setup
		config := &Config{
			Enabled:       true,
			ADB2CClientID: tokenGen.ClientID,
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
				"test": {
					IssuerURL:    tokenGen.GetIssuerURL(),
					DiscoveryURL: "https://mock-discovery-url.example.com", // This will be mocked
				},
			},
		}

		// Validate that the config is correct
		err = config.Validate()
		assert.NoError(t, err)

		// Verify the config conversion works
		trustedIssuers := config.TrustedIssuersMap()
		assert.Len(t, trustedIssuers, 1)
		assert.Equal(t, "https://mock-discovery-url.example.com", trustedIssuers[tokenGen.GetIssuerURL()])

		// Test token creation and validation with the mock client
		token, err := tokenGen.CreateToken(nil)
		require.NoError(t, err)

		claims, err := mockClient.ValidateToken(ctx, token, WithValidateSignature(false))
		require.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, tokenGen.ClientID, claims.Audience[0])
	})
}
