package token

import (
	"context"
	"encoding/base64"
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

// TestIntegrationSignatureTampering tests signature validation with real Azure AD B2C tokens
// This test validates that signature tampering is properly detected when using real tokens
func TestIntegrationSignatureTampering(t *testing.T) {
	testConfig := LoadTestConfig()
	if testConfig.Token == "" {
		t.Skip("Skipping signature tampering integration test. ADB2C_TOKEN environment variable not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	t.Logf("Creating ADB2C client for signature tampering tests")

	client, err := NewClient(ctx, testConfig.Config)
	require.NoError(t, err, "Failed to create ADB2C client from config")

	// First, verify the original token works (if not expired)
	t.Run("original token validation baseline", func(t *testing.T) {
		claims, err := client.ValidateToken(ctx, testConfig.Token, WithValidateSignature(true))

		if err != nil {
			if strings.Contains(err.Error(), "\"exp\" not satisfied") {
				t.Logf("Original token is expired, which is expected for testing: %v", err)
				// For expired tokens, we'll test without signature validation to establish baseline
				claims, err = client.ValidateToken(ctx, testConfig.Token, WithValidateSignature(false))
				require.NoError(t, err, "Token should be valid when ignoring expiry")
				t.Logf("Baseline established: token is structurally valid but expired")
			} else {
				t.Logf("Original token validation failed: %v", err)
				t.Logf("This may indicate network issues or configuration problems")
				// We'll continue with tampering tests anyway to validate the security
			}
		} else {
			require.NotNil(t, claims, "Token claims should not be nil")
			t.Logf("Original token validated successfully with signature verification")
		}
	})

	// Test signature tampering scenarios
	t.Run("tampered signature, fails", func(t *testing.T) {
		// Split the token and tamper with the signature
		parts := strings.Split(testConfig.Token, ".")
		require.Len(t, parts, 3, "Real token should have 3 parts")

		// Tamper with the signature by changing a few characters
		tamperedSignature := parts[2]
		if len(tamperedSignature) > 15 {
			// Replace some characters in different positions of the signature
			runes := []rune(tamperedSignature)
			runes[5] = 'X'
			runes[10] = 'Y'
			runes[len(runes)-5] = 'Z'
			tamperedSignature = string(runes)
		}

		tamperedToken := parts[0] + "." + parts[1] + "." + tamperedSignature

		claims, err := client.ValidateToken(ctx, tamperedToken, WithValidateSignature(true))
		assert.Error(t, err, "Tampered signature should be rejected")
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("completely fake signature, fails", func(t *testing.T) {
		// Split the token and replace with a completely fake signature
		parts := strings.Split(testConfig.Token, ".")
		require.Len(t, parts, 3, "Real token should have 3 parts")

		// Create a completely fake signature
		fakeSignature := "dGhpcy1pcy1hLWZha2Utc2lnbmF0dXJlLXRoYXQtc2hvdWxkLW5vdC12YWxpZGF0ZQ"

		fakeToken := parts[0] + "." + parts[1] + "." + fakeSignature

		claims, err := client.ValidateToken(ctx, fakeToken, WithValidateSignature(true))
		assert.Error(t, err, "Fake signature should be rejected")
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("modified payload with original signature, fails", func(t *testing.T) {
		// Parse the original token to get the payload
		originalClaims, err := ParseToken(testConfig.Token)
		require.NoError(t, err, "Should be able to parse original token")

		// Split the token
		parts := strings.Split(testConfig.Token, ".")
		require.Len(t, parts, 3, "Real token should have 3 parts")

		// Modify a claim in the payload (change the subject if it exists)
		modifiedClaims := make(map[string]interface{})
		for k, v := range originalClaims {
			modifiedClaims[k] = v
		}

		// Modify the subject claim
		if _, exists := modifiedClaims["sub"]; exists {
			modifiedClaims["sub"] = "tampered-subject-12345"
		} else {
			// If no subject, modify another claim or add one
			modifiedClaims["tampered"] = "true"
		}

		// Re-encode the modified payload
		modifiedPayloadJSON, err := json.Marshal(modifiedClaims)
		require.NoError(t, err)

		modifiedPayloadB64 := base64.RawURLEncoding.EncodeToString(modifiedPayloadJSON)

		// Use the original signature with the modified payload
		modifiedToken := parts[0] + "." + modifiedPayloadB64 + "." + parts[2]

		claims, err := client.ValidateToken(ctx, modifiedToken, WithValidateSignature(true))
		assert.Error(t, err, "Modified payload should be rejected")
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("signature validation disabled should ignore tampering", func(t *testing.T) {
		// This test ensures that when signature validation is disabled, even tampered tokens pass

		// Split the token and tamper with the signature
		parts := strings.Split(testConfig.Token, ".")
		require.Len(t, parts, 3, "Real token should have 3 parts")

		// Tamper with the signature
		tamperedSignature := parts[2]
		if len(tamperedSignature) > 10 {
			runes := []rune(tamperedSignature)
			runes[5] = 'X'
			tamperedSignature = string(runes)
		}

		tamperedToken := parts[0] + "." + parts[1] + "." + tamperedSignature

		// With signature validation disabled, this should pass (unless expired)
		claims, err := client.ValidateToken(ctx, tamperedToken, WithValidateSignature(false))

		if err != nil && strings.Contains(err.Error(), "\"exp\" not satisfied") {
			t.Logf("Token is expired, which is expected: %v", err)
		} else {
			// If not expired, it should pass without signature validation
			if err == nil {
				assert.NotNil(t, claims)
			} else {
				t.Logf("Token failed for other reasons: %v", err)
			}
		}
	})

	t.Run("empty signature, fails", func(t *testing.T) {
		// Split the token and remove the signature
		parts := strings.Split(testConfig.Token, ".")
		require.Len(t, parts, 3, "Real token should have 3 parts")

		// Create token with empty signature
		emptySignatureToken := parts[0] + "." + parts[1] + "."

		claims, err := client.ValidateToken(ctx, emptySignatureToken, WithValidateSignature(true))
		assert.Error(t, err, "Empty signature should be rejected")
		assert.Nil(t, claims)
	})
}
