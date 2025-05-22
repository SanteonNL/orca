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

// TestIntegrationWithRealToken performs an integration test with a real Azure AD B2C token
// It requires environment variables to be set with the token and Azure AD B2C details
//
// To run this test, export the required environment variables:
//
//	export ADB2C_TOKEN="your-real-token"
//	export ADB2C_CLIENT_ID="your-client-id" - this is not mandatory, and will be read from the token if not provided
//	export ADB2C_TRUSTED_ISSUERS="trusted-issuer-url-1=discovery-endpoint-url-1;trusted-issuer-url-2=discovery-endpoint-url-2" - this is not mandatory, and will be read from the token if not provided
//
// Note: This test will be skipped if the ADB2C_TOKEN environment variable is not set
func TestIntegrationWithRealToken(t *testing.T) {
	token := os.Getenv("ADB2C_TOKEN")

	if token == "" {
		t.Skip("Skipping integration test. ADB2C_TOKEN environment variable not set.")
	}

	// Get configuration from the token and environment
	issuer, clientID, trustedIssuersRaw := getTokenConfig(t, token)

	trustedIssuers, err := ParseTrustedIssuers(trustedIssuersRaw)
	require.NoError(t, err, "Failed to parse trusted issuers")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, trustedIssuers, clientID, WithDefaultIssuer(issuer))
	require.NoError(t, err, "Failed to create ADB2C client")

	claims, err := client.ValidateToken(ctx, token)

	if err != nil {
		// Expired tokens are expected in some test cases
		if errors.Is(err, ErrTokenExpired) {
			t.Logf("Token validation failed as expected: %v", err)
			t.Logf("This is normal if you're using an expired token for testing")
			return
		}
		t.Fatalf("Token validation failed: %v", err)
	}

	// If validation succeeded, verify and print the claims
	require.NotNil(t, claims, "Token claims should not be nil")

	prettyJSON, err := json.MarshalIndent(claims, "", "  ")
	require.NoError(t, err)
	t.Logf("Token validation succeeded with claims: \n%s", string(prettyJSON))

	// Verify that the subject claim matches what we parsed earlier
	parsedClaims, _ := ParseToken(token)
	assert.Equal(t, parsedClaims["sub"], claims.Subject)
}

// getTokenConfig extracts configuration from environment variables and token claims
func getTokenConfig(t *testing.T, token string) (string, string, string) {
	// Parse the token to inspect its structure
	parsedClaims, err := ParseToken(token)
	require.NoError(t, err, "Failed to parse token")

	// Print the token claims for inspection
	prettyJSON, err := json.MarshalIndent(parsedClaims, "", "  ")
	require.NoError(t, err)
	t.Logf("Token claims before validation: \n%s", string(prettyJSON))

	// Get issuer from token
	issuer := ""
	if parsedClaims["iss"] != nil {
		issuer = parsedClaims["iss"].(string)
		t.Logf("Using issuer from token: %s", issuer)
	} else {
		t.Fatal("No issuer claim found in token")
	}

	// Extract client ID from the audience claim or environment
	clientID := os.Getenv("ADB2C_CLIENT_ID")
	if clientID == "" && parsedClaims["aud"] != nil {
		// The audience claim can be a string or an array
		switch aud := parsedClaims["aud"].(type) {
		case string:
			clientID = aud
		case []interface{}:
			if len(aud) > 0 {
				clientID = aud[0].(string)
			}
		}
		t.Logf("Extracted client ID from token: %s", clientID)
	}

	// Get trusted issuers configuration
	trustedIssuersRaw := os.Getenv("ADB2C_TRUSTED_ISSUERS")
	if trustedIssuersRaw == "" {
		// Construct from token claims
		tfp := ""
		if parsedClaims["tfp"] != nil {
			tfp = parsedClaims["tfp"].(string)
			t.Logf("Using tfp from token: %s", tfp)
		} else {
			t.Fatal("No tfp claim found in token")
		}

		discoveryURL := strings.Replace(issuer, "/v2.0/", fmt.Sprintf("/%s/v2.0/.well-known/openid-configuration", tfp), 1)
		t.Logf("Using discovery URL: %s", discoveryURL)
		trustedIssuersRaw = fmt.Sprintf("%s=%s", issuer, discoveryURL)
	}

	return issuer, clientID, trustedIssuersRaw
}
