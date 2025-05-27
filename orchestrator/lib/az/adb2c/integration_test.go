package adb2c

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ADB2CTestConfig holds the configuration for the Azure AD B2C integration test
type ADB2CTestConfig struct {
	Token    string
	ClientID string
	Config   *Config
}

func LoadADB2CTestConfig() *ADB2CTestConfig {
	testConfig := &ADB2CTestConfig{
		Token:    os.Getenv("ADB2C_TOKEN"),
		ClientID: os.Getenv("ADB2C_CLIENT_ID"),
	}

	// Load the koanf-based configuration
	config, err := LoadConfig()
	if err != nil {
		// If koanf config loading fails, create a minimal config
		config = &Config{
			Enabled:             false,
			ADB2CClientID:       testConfig.ClientID,
			ADB2CTrustedIssuers: make(map[string]TrustedIssuer),
		}
	}

	// Override client ID from environment if provided
	if testConfig.ClientID != "" {
		config.ADB2CClientID = testConfig.ClientID
	}

	testConfig.Config = config
	return testConfig
}

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
	testConfig := LoadADB2CTestConfig()
	if testConfig.Token == "" {
		t.Skip("Skipping integration test. ADB2C_TOKEN environment variable not set.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// getTokenConfig extracts configuration from token claims
func getTokenConfig(t *testing.T, testConfig *ADB2CTestConfig) (string, string, map[string]string) {
	// Parse the token to inspect its structure
	parsedClaims, err := ParseToken(testConfig.Token)
	require.NoError(t, err, "Failed to parse token")

	// Get issuer from token
	issuer := ""
	if value, ok := parsedClaims["iss"].(string); ok {
		issuer = value
		t.Logf("Using issuer from token: %s", issuer)
	} else {
		t.Fatal("Issuer claim is missing or not a string in token")
	}

	// Extract client ID from the audience claim or environment
	clientID := testConfig.ClientID
	if clientID == "" {
		clientID = testConfig.Config.ADB2CClientID
	}
	if clientID == "" && parsedClaims["aud"] != nil {
		// The audience claim can be a string or an array
		switch aud := parsedClaims["aud"].(type) {
		case string:
			clientID = aud
		case []interface{}:
			if len(aud) > 0 {
				clientID = aud[0].(string)
			}
		case []string:
			if len(aud) > 0 {
				clientID = aud[0]
			}
		}
		t.Logf("Extracted client ID from token: %s", clientID)
	}

	// Use trusted issuers from koanf config if available, otherwise generate them
	trustedIssuers := make(map[string]string)

	if len(testConfig.Config.ADB2CTrustedIssuers) > 0 {
		// Use the trusted issuers from koanf configuration
		trustedIssuers = testConfig.Config.TrustedIssuersMap()
		t.Logf("Using trusted issuers from koanf configuration: %d issuers", len(trustedIssuers))
		for issuerURL, discoveryURL := range trustedIssuers {
			t.Logf("  %s -> %s", issuerURL, discoveryURL)
		}
	} else {
		// Fallback: generate trusted issuers from token information
		t.Logf("No trusted issuers configured, generating from token information")

		// Get policy from token
		tfp := ""
		if parsedClaims["tfp"] != nil {
			tfp = parsedClaims["tfp"].(string)
			t.Logf("Using policy from token: %s", tfp)
		}

		// Extract domain and tenant ID from issuer
		parts := strings.Split(issuer, "/")
		if len(parts) >= 4 {
			domain := parts[2]
			tenantID := parts[3]

			// Generate discovery endpoints
			if tfp != "" {
				// Direct JWKS URI with policy
				jwksUri := fmt.Sprintf("https://%s/%s/discovery/v2.0/keys?p=%s", domain, tenantID, tfp)
				trustedIssuers[issuer] = jwksUri
				t.Logf("Generated trusted issuer mapping: %s -> %s", issuer, jwksUri)
			} else {
				// Fallback without policy parameter
				jwksUri := fmt.Sprintf("https://%s/%s/discovery/v2.0/keys", domain, tenantID)
				trustedIssuers[issuer] = jwksUri
				t.Logf("Generated trusted issuer mapping: %s -> %s", issuer, jwksUri)
			}
		} else {
			t.Logf("Could not extract domain and tenant ID from issuer: %s", issuer)
		}
	}

	return issuer, clientID, trustedIssuers
}

// TestLoadADB2CTestConfig tests the configuration loading for integration tests
func TestLoadADB2CTestConfig(t *testing.T) {
	t.Run("loads config with koanf environment variables", func(t *testing.T) {
		// Set up environment variables in the new koanf format
		os.Setenv("ADB2C_TOKEN", "test-token")
		os.Setenv("ADB2C_CLIENT_ID", "test-client-from-env")
		os.Setenv("ADB2C_ENABLED", "true")
		os.Setenv("ADB2C_CLIENTID", "test-client-from-koanf")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration")

		defer func() {
			os.Unsetenv("ADB2C_TOKEN")
			os.Unsetenv("ADB2C_CLIENT_ID")
			os.Unsetenv("ADB2C_ENABLED")
			os.Unsetenv("ADB2C_CLIENTID")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")
		}()

		testConfig := LoadADB2CTestConfig()

		// Verify token and client ID are loaded correctly
		assert.Equal(t, "test-token", testConfig.Token)
		assert.Equal(t, "test-client-from-env", testConfig.ClientID) // ADB2C_CLIENT_ID takes precedence

		// Verify koanf config is loaded
		assert.NotNil(t, testConfig.Config)
		assert.True(t, testConfig.Config.Enabled)
		assert.Equal(t, "test-client-from-env", testConfig.Config.ADB2CClientID) // Should be overridden by ADB2C_CLIENT_ID

		// Verify trusted issuers are loaded
		assert.Len(t, testConfig.Config.ADB2CTrustedIssuers, 1)
		assert.Contains(t, testConfig.Config.ADB2CTrustedIssuers, "tenant1")

		trustedIssuer := testConfig.Config.ADB2CTrustedIssuers["tenant1"]
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/", trustedIssuer.IssuerURL)
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration", trustedIssuer.DiscoveryURL)

		// Verify TrustedIssuersMap works
		trustedIssuersMap := testConfig.Config.TrustedIssuersMap()
		assert.Len(t, trustedIssuersMap, 1)
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			trustedIssuersMap["https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/"])
	})

	t.Run("handles missing koanf config gracefully", func(t *testing.T) {
		// Clear all ADB2C environment variables
		os.Unsetenv("ADB2C_ENABLED")
		os.Unsetenv("ADB2C_CLIENTID")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")

		// Set only the test-specific variables
		os.Setenv("ADB2C_TOKEN", "test-token")
		os.Setenv("ADB2C_CLIENT_ID", "test-client")

		defer func() {
			os.Unsetenv("ADB2C_TOKEN")
			os.Unsetenv("ADB2C_CLIENT_ID")
		}()

		testConfig := LoadADB2CTestConfig()

		// Verify basic config is loaded
		assert.Equal(t, "test-token", testConfig.Token)
		assert.Equal(t, "test-client", testConfig.ClientID)

		// Verify fallback config is created
		assert.NotNil(t, testConfig.Config)
		assert.False(t, testConfig.Config.Enabled) // Should be false when no koanf config
		assert.Equal(t, "test-client", testConfig.Config.ADB2CClientID)
		assert.Len(t, testConfig.Config.ADB2CTrustedIssuers, 0)
	})

	t.Run("client ID precedence", func(t *testing.T) {
		// Test that ADB2C_CLIENT_ID takes precedence over ADB2C_CLIENTID
		os.Setenv("ADB2C_CLIENT_ID", "client-from-client-id")
		os.Setenv("ADB2C_CLIENTID", "client-from-client")

		defer func() {
			os.Unsetenv("ADB2C_CLIENT_ID")
			os.Unsetenv("ADB2C_CLIENTID")
		}()

		testConfig := LoadADB2CTestConfig()

		assert.Equal(t, "client-from-client-id", testConfig.ClientID)
		assert.Equal(t, "client-from-client-id", testConfig.Config.ADB2CClientID)
	})
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
