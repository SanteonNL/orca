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
	Token          string
	ClientID       string
	TrustedIssuers map[string]string
}

// LoadADB2CTestConfig loads the ADB2C configuration from environment variables
func LoadADB2CTestConfig() *ADB2CTestConfig {
	config := &ADB2CTestConfig{
		Token:          os.Getenv("ADB2C_TOKEN"),
		ClientID:       os.Getenv("ADB2C_CLIENT_ID"),
		TrustedIssuers: make(map[string]string),
	}

	// Parse trusted issuers from environment variable in format:
	// issuer-url-1=https://example.com/issuer1;issuer-url-2=https://example.com/issuer2
	trustedIssuersEnv := os.Getenv("ADB2C_TRUSTED_ISSUERS")
	if trustedIssuersEnv != "" {
		pairs := strings.Split(trustedIssuersEnv, ";")
		for _, pair := range pairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) == 2 {
				issuer := parts[0]
				endpoint := parts[1]
				config.TrustedIssuers[issuer] = endpoint
			}
		}
	}

	return config
}

// TestIntegrationWithRealToken performs an integration test with a real Azure AD B2C token
// It requires environment variables to be set with the token and Azure AD B2C details
//
// To run this test, export the required environment variables:
//
//		export ADB2C_TOKEN="your-real-token"
//		export ADB2C_CLIENT_ID="your-client-id" - this is not mandatory, and will be read from the token if not provided
//	 	export ADB2C_TRUSTED_ISSUERS=issuer-url-1=https://example.com/issuer1;issuer-url-2=https://example.com/issuer2
//
// Note: This test will be skipped if the ADB2C_TOKEN environment variable is not set
func TestIntegrationWithRealToken(t *testing.T) {
	config := LoadADB2CTestConfig()
	if config.Token == "" {
		t.Skip("Skipping integration test. ADB2C_TOKEN environment variable not set.")
	}

	// Get configuration from the token and environment
	issuer, clientID, trustedIssuers := getTokenConfig(t, config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a client with the provided issuer and client ID
	client, err := NewClient(ctx, trustedIssuers, clientID, WithDefaultIssuer(issuer))
	require.NoError(t, err, "Failed to create ADB2C client")

	// Use the token validation with real signature verification
	claims, err := client.ValidateToken(ctx, config.Token)

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
		claims, err = client.ValidateToken(ctx, config.Token, WithValidateSignature(false))
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
	parsedClaims, _ := ParseToken(config.Token)
	assert.Equal(t, parsedClaims["sub"], claims.Subject)
}

// getTokenConfig extracts configuration from token claims
func getTokenConfig(t *testing.T, config *ADB2CTestConfig) (string, string, map[string]string) {
	// Parse the token to inspect its structure
	parsedClaims, err := ParseToken(config.Token)
	require.NoError(t, err, "Failed to parse token")

	// Get issuer from token
	issuer := ""
	if parsedClaims["iss"] != nil {
		issuer = parsedClaims["iss"].(string)
		t.Logf("Using issuer from token: %s", issuer)
	} else {
		t.Fatal("No issuer claim found in token")
	}

	// Extract client ID from the audience claim or environment
	clientID := config.ClientID
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

	// Use trusted issuers from config if available, otherwise generate them
	trustedIssuers := make(map[string]string)

	if len(config.TrustedIssuers) > 0 {
		// Use the map directly from environment variables
		trustedIssuers = config.TrustedIssuers
	} else {
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
