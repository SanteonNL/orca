package adb2c

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/v2.0/"
	testClientID = "test-client"
)

func TestNewClient(t *testing.T) {
	// Create a mock OpenID configuration server
	openIDConfig := OpenIDConfig{
		Issuer:                testIssuer,
		JwksURI:               "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/discovery/v2.0/keys",
		AuthorizationEndpoint: "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/authorize",
		TokenEndpoint:         "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/token",
	}

	openIDConfigJSON, err := json.Marshal(openIDConfig)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(openIDConfigJSON)
	}))
	defer server.Close()

	tests := []struct {
		name      string
		issuers   map[string]string
		clientID  string
		options   []ClientOption
		wantError bool
	}{
		{
			name: "Valid parameters",
			issuers: map[string]string{
				testIssuer: server.URL,
			},
			clientID:  testClientID,
			options:   []ClientOption{WithDefaultIssuer(testIssuer)},
			wantError: false,
		},
		{
			name:      "Empty issuers",
			issuers:   map[string]string{},
			clientID:  testClientID,
			wantError: true,
		},
		{
			name: "Invalid discovery URL",
			issuers: map[string]string{
				testIssuer: "http://invalid-url",
			},
			clientID:  testClientID,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client, err := NewClient(ctx, tt.issuers, tt.clientID, tt.options...)
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, testIssuer, client.Issuer)
				assert.Equal(t, tt.clientID, client.ClientID)
			}
		})
	}
}

func TestMetadataRefresh(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create a mock JWK server
	jwkSet := jwk.NewSet()
	jwkKey, err := jwk.New(publicKey)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)
	jwkSet.Add(jwkKey)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer.Close()

	// Create a counter to track OpenID configuration requests
	requestCount := 0

	// Create a mock OpenID configuration server that changes its response
	openIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		requestCount++

		// First request returns original JWKS URI
		if requestCount == 1 {
			config := OpenIDConfig{
				Issuer:                testIssuer,
				JwksURI:               jwkServer.URL,
				AuthorizationEndpoint: "https://original.endpoint/authorize",
				TokenEndpoint:         "https://original.endpoint/token",
			}
			json.NewEncoder(w).Encode(config)
		} else {
			// Subsequent requests return updated JWKS URI
			config := OpenIDConfig{
				Issuer:                testIssuer,
				JwksURI:               jwkServer.URL + "/updated",
				AuthorizationEndpoint: "https://updated.endpoint/authorize",
				TokenEndpoint:         "https://updated.endpoint/token",
			}
			json.NewEncoder(w).Encode(config)
		}
	}))
	defer openIDServer.Close()

	// Create a test client with the mock servers and a short refresh interval
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := NewClient(
		ctx,
		trustedIssuers,
		testClientID,
		WithDefaultIssuer(testIssuer),
		WithRefreshInterval(5*time.Millisecond), // Short refresh interval for testing
	)
	require.NoError(t, err)

	// Initial configuration should be set
	assert.Equal(t, jwkServer.URL, client.JwksURI)

	// Wait for refresh interval to pass
	time.Sleep(10 * time.Millisecond)

	// Create a valid token
	validToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "test-key-id")

	// Validate token to trigger refresh
	_, err = client.ValidateToken(ctx, validToken)
	require.NoError(t, err)

	// Check that configuration was refreshed
	c := client
	c.issuerConfigsMutex.RLock()
	config := c.issuerConfigs[testIssuer]
	c.issuerConfigsMutex.RUnlock()

	assert.Equal(t, jwkServer.URL+"/updated", config.JwksURI)
	assert.Equal(t, "https://updated.endpoint/authorize", config.AuthorizationEndpoint)
	assert.True(t, requestCount >= 2, "Expected at least 2 requests to OpenID configuration endpoint")
}

func TestValidateToken(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create a mock JWK server
	jwkSet := jwk.NewSet()
	jwkKey, err := jwk.New(publicKey)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)
	jwkSet.Add(jwkKey)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := OpenIDConfig{
		Issuer:                testIssuer,
		JwksURI:               jwkServer.URL,
		AuthorizationEndpoint: "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/authorize",
		TokenEndpoint:         "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/token",
	}

	openIDConfigJSON, err := json.Marshal(openIDConfig)
	require.NoError(t, err)

	openIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(openIDConfigJSON)
	}))
	defer openIDServer.Close()

	// Create a test client with the mock servers
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := NewClient(ctx, trustedIssuers, testClientID, WithDefaultIssuer(testIssuer))
	require.NoError(t, err)

	// Create a valid token
	validToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss":    testIssuer,
		"aud":    testClientID,
		"sub":    "test-subject",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Add(-time.Minute).Unix(),
		"name":   "Test User",
		"emails": []string{"test.user@example.com"},
		"roles":  []string{"User", "Admin"},
	}, "test-key-id")

	// Create an expired token
	expiredToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(-time.Hour).Unix(),
		"iat": time.Now().Add(-time.Hour * 2).Unix(),
	}, "test-key-id")

	// Create a token with invalid issuer
	invalidIssuerToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": "https://invalid-issuer.com",
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "test-key-id")

	// Create a token with invalid audience
	invalidAudienceToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": "invalid-client-id",
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "test-key-id")

	// Create a token with invalid key ID
	invalidKeyIDToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "invalid-key-id")

	tests := []struct {
		name      string
		token     string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "Valid token",
			token:     validToken,
			wantError: false,
		},
		{
			name:      "Expired token",
			token:     expiredToken,
			wantError: true,
			errorMsg:  "token has expired",
		},
		{
			name:      "Invalid issuer",
			token:     invalidIssuerToken,
			wantError: true,
			errorMsg:  "untrusted token issuer",
		},
		{
			name:      "Invalid audience",
			token:     invalidAudienceToken,
			wantError: true,
			errorMsg:  "invalid token audience",
		},
		{
			name:      "Invalid key ID",
			token:     invalidKeyIDToken,
			wantError: true,
			errorMsg:  "matching key not found in JWKS",
		},
		{
			name:      "Invalid token format",
			token:     "invalid-token",
			wantError: true,
			errorMsg:  "invalid token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := client.ValidateToken(ctx, tt.token)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, claims)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				assert.Equal(t, "test-subject", claims.Subject)
				assert.Equal(t, "Test User", claims.Name)
				assert.Equal(t, []string{"test.user@example.com"}, claims.Emails)
				assert.Equal(t, []string{"User", "Admin"}, claims.Roles)
			}
		})
	}
}

func TestKeyRotationHandling(t *testing.T) {
	// Generate two test RSA key pairs (to simulate key rotation)
	privateKey1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKey2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwkRequestCount := 0

	// Create a mock JWK server that serves both keys after the first request
	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		jwkRequestCount++

		// First request: only original key
		if jwkRequestCount == 1 {
			jwkSet := jwk.NewSet()
			jwkKey, _ := jwk.New(privateKey1.Public())
			jwkKey.Set(jwk.KeyIDKey, "original-key-id")
			jwkSet.Add(jwkKey)
			json.NewEncoder(w).Encode(jwkSet)
		} else {
			// Subsequent requests: both keys (simulating key rotation)
			jwkSet := jwk.NewSet()

			// Add the original key
			jwkKey1, _ := jwk.New(privateKey1.Public())
			jwkKey1.Set(jwk.KeyIDKey, "original-key-id")
			jwkSet.Add(jwkKey1)

			// Add the new key
			jwkKey2, _ := jwk.New(privateKey2.Public())
			jwkKey2.Set(jwk.KeyIDKey, "new-key-id")
			jwkSet.Add(jwkKey2)

			json.NewEncoder(w).Encode(jwkSet)
		}
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := OpenIDConfig{
		Issuer:                testIssuer,
		JwksURI:               jwkServer.URL,
		AuthorizationEndpoint: "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/authorize",
		TokenEndpoint:         "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/oauth2/v2.0/token",
	}

	openIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(openIDConfig)
	}))
	defer openIDServer.Close()

	// Create a test client with the mock servers
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := NewClient(
		ctx,
		trustedIssuers,
		testClientID,
		WithDefaultIssuer(testIssuer),
	)
	require.NoError(t, err)

	// Create a token signed with the original key
	originalToken := createTestToken(t, privateKey1, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "original-key-id")

	// Validate the token - should succeed
	claims, err := client.ValidateToken(ctx, originalToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	// Create a token signed with the new key
	newToken := createTestToken(t, privateKey2, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject-2",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "new-key-id")

	// Validate the token with the new key
	// This should fail initially because the key is not in the JWKS yet
	_, firstErr := client.ValidateToken(ctx, newToken)
	assert.Error(t, firstErr)
	assert.Contains(t, firstErr.Error(), "matching key not found in JWKS")

	// Verify that we've made at least one request to the JWKS endpoint
	assert.GreaterOrEqual(t, jwkRequestCount, 1)

	// Now force a refresh of the JWKS
	// In a real scenario, this would happen automatically when the token validation fails
	// and the client attempts to refresh the JWKS
	jwks, err := client.jwksFetcher.Refresh(ctx, jwkServer.URL)
	require.NoError(t, err)
	require.NotNil(t, jwks)

	// Verify that we've made another request to the JWKS endpoint
	assert.GreaterOrEqual(t, jwkRequestCount, 2)

	// Now validation should succeed with the new key
	claims, err = client.ValidateToken(ctx, newToken)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "test-subject-2", claims.Subject)
}

func TestMultipleTrustedIssuers(t *testing.T) {
	// Generate test RSA key pairs for each issuer
	privateKey1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKey2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create mock JWK servers for each issuer
	jwkServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwkSet := jwk.NewSet()
		jwkKey, _ := jwk.New(privateKey1.Public())
		jwkKey.Set(jwk.KeyIDKey, "issuer1-key-id")
		jwkSet.Add(jwkKey)
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer1.Close()

	jwkServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwkSet := jwk.NewSet()
		jwkKey, _ := jwk.New(privateKey2.Public())
		jwkKey.Set(jwk.KeyIDKey, "issuer2-key-id")
		jwkSet.Add(jwkKey)
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer2.Close()

	// Define two different issuers
	issuer1 := "https://issuer1.example.com/v2.0/"
	issuer2 := "https://issuer2.example.com/v2.0/"

	// Create mock OpenID configuration servers for each issuer
	openIDServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		config := OpenIDConfig{
			Issuer:                issuer1,
			JwksURI:               jwkServer1.URL,
			AuthorizationEndpoint: "https://issuer1.example.com/oauth2/v2.0/authorize",
			TokenEndpoint:         "https://issuer1.example.com/oauth2/v2.0/token",
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer openIDServer1.Close()

	openIDServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		config := OpenIDConfig{
			Issuer:                issuer2,
			JwksURI:               jwkServer2.URL,
			AuthorizationEndpoint: "https://issuer2.example.com/oauth2/v2.0/authorize",
			TokenEndpoint:         "https://issuer2.example.com/oauth2/v2.0/token",
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer openIDServer2.Close()

	// Create a client with multiple trusted issuers
	trustedIssuers := map[string]string{
		issuer1: openIDServer1.URL,
		issuer2: openIDServer2.URL,
	}

	ctx := context.Background()
	client, err := NewClient(ctx, trustedIssuers, testClientID, WithDefaultIssuer(issuer1))
	require.NoError(t, err)

	// Create tokens from each issuer
	token1 := createTestToken(t, privateKey1, map[string]interface{}{
		"iss":    issuer1,
		"aud":    testClientID,
		"sub":    "subject-from-issuer1",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Add(-time.Minute).Unix(),
		"name":   "User from Issuer 1",
		"emails": []string{"user1@example.com"},
	}, "issuer1-key-id")

	token2 := createTestToken(t, privateKey2, map[string]interface{}{
		"iss":    issuer2,
		"aud":    testClientID,
		"sub":    "subject-from-issuer2",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Add(-time.Minute).Unix(),
		"name":   "User from Issuer 2",
		"emails": []string{"user2@example.com"},
	}, "issuer2-key-id")

	// Validate token from issuer 1
	claims1, err := client.ValidateToken(ctx, token1)
	require.NoError(t, err)
	require.NotNil(t, claims1)
	assert.Equal(t, "subject-from-issuer1", claims1.Subject)
	assert.Equal(t, "User from Issuer 1", claims1.Name)
	assert.Equal(t, []string{"user1@example.com"}, claims1.Emails)

	// Validate token from issuer 2
	claims2, err := client.ValidateToken(ctx, token2)
	require.NoError(t, err)
	require.NotNil(t, claims2)
	assert.Equal(t, "subject-from-issuer2", claims2.Subject)
	assert.Equal(t, "User from Issuer 2", claims2.Name)
	assert.Equal(t, []string{"user2@example.com"}, claims2.Emails)

	// Create a token with an untrusted issuer
	untrustedToken := createTestToken(t, privateKey1, map[string]interface{}{
		"iss": "https://untrusted-issuer.com/v2.0/",
		"aud": testClientID,
		"sub": "subject-from-untrusted",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "issuer1-key-id")

	// Validate token from untrusted issuer - should fail
	claims3, err := client.ValidateToken(ctx, untrustedToken)
	assert.Error(t, err)
	assert.Nil(t, claims3)
	assert.Contains(t, err.Error(), "untrusted token issuer")
}

func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]interface{}, keyID string) string {
	tokenString, err := CreateTestTokenWithClaims(privateKey, claims, keyID)
	require.NoError(t, err)
	return tokenString
}

func TestParseTrustedIssuers(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]string
		expectError bool
	}{
		{
			name:  "valid multiple issuers",
			input: "issuer1=https://issuer1.com/.well-known/openid-configuration;issuer2=https://issuer2.com/.well-known/openid-configuration",
			expected: map[string]string{
				"issuer1": "https://issuer1.com/.well-known/openid-configuration",
				"issuer2": "https://issuer2.com/.well-known/openid-configuration",
			},
			expectError: false,
		},
		{
			name:        "empty string",
			input:       "",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid format missing equals",
			input:       "issuer1;issuer2",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty issuer",
			input:       "=https://issuer1.com/.well-known/openid-configuration",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "empty discovery URL",
			input:       "issuer1=",
			expected:    nil,
			expectError: true,
		},
		{
			name:  "single issuer",
			input: "issuer1=https://issuer1.com/.well-known/openid-configuration",
			expected: map[string]string{
				"issuer1": "https://issuer1.com/.well-known/openid-configuration",
			},
			expectError: false,
		},
		{
			name:  "whitespace handling",
			input: " issuer1 = https://issuer1.com/.well-known/openid-configuration ; issuer2 = https://issuer2.com/.well-known/openid-configuration ",
			expected: map[string]string{
				"issuer1": "https://issuer1.com/.well-known/openid-configuration",
				"issuer2": "https://issuer2.com/.well-known/openid-configuration",
			},
			expectError: false,
		},
		{
			name:        "no valid pairs",
			input:       ";;;",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTrustedIssuers(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
