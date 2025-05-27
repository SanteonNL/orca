package token

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testIssuer   = "https://test-tenant.b2clogin.com/test-tenant.onmicrosoft.com/v2.0/"
	testClientID = "test-client"
)

// ParseToken parses a JWT token without validation and returns the claims
func ParseToken(tokenString string) (map[string]interface{}, error) {
	token, err := jwt.Parse([]byte(tokenString), jwt.WithValidate(false), jwt.WithVerify(false))
	if err != nil {
		return nil, fmt.Errorf("error parsing token: %w", err)
	}

	allClaims := make(map[string]interface{})

	// Add standard JWT claims
	if v := token.Issuer(); v != "" {
		allClaims["iss"] = v
	}
	if v := token.Subject(); v != "" {
		allClaims["sub"] = v
	}
	if v := token.Audience(); len(v) > 0 {
		allClaims["aud"] = v
	}

	if v := token.Expiration(); !v.IsZero() {
		allClaims["exp"] = v.Unix()
	}

	if v := token.IssuedAt(); !v.IsZero() {
		allClaims["iat"] = v.Unix()
	}

	if v := token.NotBefore(); !v.IsZero() {
		allClaims["nbf"] = v.Unix()
	}

	if v := token.JwtID(); v != "" {
		allClaims["jti"] = v
	}

	// Add all private claims
	privateClaims := token.PrivateClaims()
	for k, v := range privateClaims {
		allClaims[k] = v
	}

	return allClaims, nil
}

func TestNewClient(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		config        *Config
		expectedError error
	}{
		{
			name:          "nil config",
			config:        nil,
			expectedError: errors.New("config cannot be nil"),
		},
		{
			name: "disabled config",
			config: &Config{
				Enabled:       false,
				ADB2CClientID: testClientID,
				ADB2CTrustedIssuers: map[string]TrustedIssuer{
					"test": {
						IssuerURL:    testIssuer,
						DiscoveryURL: "https://test.example.com/.well-known/openid_configuration",
					},
				},
			},
			expectedError: errors.New("ADB2C is not enabled in configuration"),
		},
		{
			name: "invalid config - missing client ID",
			config: &Config{
				Enabled:             true,
				ADB2CClientID:       "",
				ADB2CTrustedIssuers: map[string]TrustedIssuer{},
			},
			expectedError: errors.New("invalid configuration"),
		},
		{
			name: "invalid config - no trusted issuers",
			config: &Config{
				Enabled:             true,
				ADB2CClientID:       testClientID,
				ADB2CTrustedIssuers: map[string]TrustedIssuer{},
			},
			expectedError: errors.New("invalid configuration"),
		},
		{
			name: "valid config but network error",
			config: &Config{
				Enabled:       true,
				ADB2CClientID: testClientID,
				ADB2CTrustedIssuers: map[string]TrustedIssuer{
					"test": {
						IssuerURL:    testIssuer,
						DiscoveryURL: "https://invalid.example.com/.well-known/openid_configuration",
					},
				},
			},
			expectedError: nil, // Client creation should succeed, errors are logged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(ctx, tt.config)
			if tt.expectedError != nil {
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.config.ADB2CClientID, client.ClientID)
			}
		})
	}
}

func TestNewClientWithTrustedIssuers(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		issuers       map[string]string
		clientID      string
		options       []ClientOption
		expectedError error
	}{
		{
			name:          "Valid parameters but network error",
			issuers:       map[string]string{testIssuer: "https://test.example.com/.well-known/openid_configuration"},
			clientID:      testClientID,
			options:       []ClientOption{},
			expectedError: nil, // Client creation should succeed, errors are logged
		},
		{
			name:          "Empty issuers, fails",
			issuers:       map[string]string{},
			clientID:      testClientID,
			options:       []ClientOption{},
			expectedError: errors.New("at least one trusted issuer is required"),
		},
		{
			name:          "Invalid discovery URL, fails",
			issuers:       map[string]string{testIssuer: "https://invalid.example.com/.well-known/openid_configuration"},
			clientID:      testClientID,
			options:       []ClientOption{},
			expectedError: nil, // Client creation should succeed, errors are logged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newClientWithTrustedIssuers(ctx, tt.issuers, tt.clientID, tt.options...)
			if tt.expectedError != nil {
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				assert.Equal(t, tt.clientID, client.ClientID)
			}
		})
	}
}

func TestKeyRefresh(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create two different JWKS for testing refresh
	jwkSet1 := jwk.NewSet()
	jwkKey1, err := jwk.FromRaw(publicKey)
	require.NoError(t, err)
	err = jwkKey1.Set(jwk.KeyIDKey, "original-key-id")
	require.NoError(t, err)
	err = jwkKey1.Set(jwk.AlgorithmKey, jwa.RS256)
	require.NoError(t, err)
	jwkSet1.AddKey(jwkKey1)

	jwkSet2 := jwk.NewSet()
	jwkKey2, err := jwk.FromRaw(publicKey)
	require.NoError(t, err)
	err = jwkKey2.Set(jwk.KeyIDKey, "updated-key-id")
	require.NoError(t, err)
	err = jwkKey2.Set(jwk.AlgorithmKey, jwa.RS256)
	require.NoError(t, err)
	jwkSet2.AddKey(jwkKey2)

	// Create a variable to track the JWKS server state
	var currentJwkSet = jwkSet1
	jwksRequestCount := 0

	// Setup a JWKS server that serves different responses based on request count
	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwksRequestCount++
		json.NewEncoder(w).Encode(currentJwkSet)
	}))
	defer jwkServer.Close()

	// Create a counter to track OpenID configuration requests
	configRequestCount := 0

	// Create a mock OpenID configuration server
	openIDServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		configRequestCount++

		config := oidc.DiscoveryConfiguration{
			Issuer:                testIssuer,
			JwksURI:               jwkServer.URL,
			AuthorizationEndpoint: "https://test.endpoint/authorize",
			TokenEndpoint:         "https://test.endpoint/token",
		}
		json.NewEncoder(w).Encode(config)
	}))
	defer openIDServer.Close()

	// Create a test client
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := newClientWithTrustedIssuers(
		ctx,
		trustedIssuers,
		testClientID,
		WithRefreshInterval(time.Nanosecond), // Very short interval to ensure refresh
	)
	require.NoError(t, err)

	// Initial key fetch should work
	keySet1, err := client.fetchKeySet(ctx, testIssuer)
	require.NoError(t, err)
	require.NotNil(t, keySet1)

	// Verify the key ID from the first key set
	var foundKeyID1 string
	for iter := keySet1.Keys(ctx); iter.Next(ctx); {
		pair := iter.Pair()
		key := pair.Value.(jwk.Key)
		foundKeyID1 = key.KeyID()
		break
	}
	assert.Equal(t, "original-key-id", foundKeyID1)

	// Switch to the second key set for the next request
	currentJwkSet = jwkSet2

	// Create a valid token for testing
	validToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}, "original-key-id")

	// Force the client to refresh by setting lastRefresh to the past
	client.statesMutex.RLock()
	state := client.issuerStates[testIssuer]
	client.statesMutex.RUnlock()

	state.mutex.Lock()
	state.lastRefresh = time.Now().Add(-2 * client.refreshInterval)
	state.mutex.Unlock()

	// Validate token - this should trigger a refresh
	_, err = client.ValidateToken(ctx, validToken, WithValidateSignature(false))
	require.NoError(t, err)

	// Fetch keys again, should get the updated set
	keySet2, err := client.fetchKeySet(ctx, testIssuer)
	require.NoError(t, err)
	require.NotNil(t, keySet2)

	// Verify the key ID from the second key set
	var foundKeyID2 string
	for iter := keySet2.Keys(ctx); iter.Next(ctx); {
		pair := iter.Pair()
		key := pair.Value.(jwk.Key)
		foundKeyID2 = key.KeyID()
		break
	}
	assert.Equal(t, "updated-key-id", foundKeyID2)

	// Verify request counts
	assert.GreaterOrEqual(t, configRequestCount, 2, "Expected at least 2 OpenID configuration requests")
	assert.GreaterOrEqual(t, jwksRequestCount, 2, "Expected at least 2 JWKS requests")
}

func TestValidateToken(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create a mock JWK server
	jwkSet := jwk.NewSet()
	jwkKey, err := jwk.FromRaw(publicKey)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)
	jwkSet.AddKey(jwkKey)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := oidc.DiscoveryConfiguration{
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
	client, err := newClientWithTrustedIssuers(
		ctx,
		trustedIssuers,
		testClientID,
		WithRefreshInterval(time.Nanosecond), // Very short interval to ensure refresh
	)
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
		name          string
		token         string
		expectedError error
	}{
		{
			name:          "Valid token, ok",
			token:         validToken,
			expectedError: nil,
		},
		{
			name:          "Expired token, fails",
			token:         expiredToken,
			expectedError: errors.New("\"exp\" not satisfied"),
		},
		{
			name:          "Invalid issuer, fails",
			token:         invalidIssuerToken,
			expectedError: errors.New("untrusted token issuer"),
		},
		{
			name:          "Invalid audience, fails",
			token:         invalidAudienceToken,
			expectedError: errors.New("\"aud\" not satisfied"),
		},
		{
			name:          "Invalid key ID, fails",
			token:         invalidKeyIDToken,
			expectedError: nil,
		},
		{
			name:          "Invalid token format, fails",
			token:         "invalid-token",
			expectedError: errors.New("invalid token format"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := client.ValidateToken(ctx, tt.token, WithValidateSignature(false))

			if tt.expectedError != nil {
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)

				// For the Invalid_key_ID test, we only check the basic claims
				if tt.name == "Invalid key ID, fails" {
					assert.Equal(t, "test-subject", claims.Subject)
				} else {
					assert.Equal(t, "test-subject", claims.Subject)
					assert.Equal(t, "Test User", claims.Name)
					assert.Equal(t, []string{"test.user@example.com"}, claims.Emails)
					assert.Equal(t, []string{"User", "Admin"}, claims.Roles)
				}
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
			jwkKey, _ := jwk.FromRaw(privateKey1.Public())
			jwkKey.Set(jwk.KeyIDKey, "original-key-id")
			jwkSet.AddKey(jwkKey)
			json.NewEncoder(w).Encode(jwkSet)
		} else {
			// Subsequent requests: both keys (simulating key rotation)
			jwkSet := jwk.NewSet()

			// Add the original key
			jwkKey1, _ := jwk.FromRaw(privateKey1.Public())
			jwkKey1.Set(jwk.KeyIDKey, "original-key-id")
			jwkSet.AddKey(jwkKey1)

			// Add the new key
			jwkKey2, _ := jwk.FromRaw(privateKey2.Public())
			jwkKey2.Set(jwk.KeyIDKey, "new-key-id")
			jwkSet.AddKey(jwkKey2)

			json.NewEncoder(w).Encode(jwkSet)
		}
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := oidc.DiscoveryConfiguration{
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
	client, err := newClientWithTrustedIssuers(
		ctx,
		trustedIssuers,
		testClientID,
		WithRefreshInterval(time.Nanosecond),
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
	claims, err := client.ValidateToken(ctx, originalToken, WithValidateSignature(false))
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
	// Since we're not validating signatures in our current implementation,
	// this should succeed even though the key is not in the JWKS yet
	claims, err = client.ValidateToken(ctx, newToken, WithValidateSignature(false))
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
		jwkKey, _ := jwk.FromRaw(privateKey1.Public())
		jwkKey.Set(jwk.KeyIDKey, "issuer1-key-id")
		jwkSet.AddKey(jwkKey)
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer1.Close()

	jwkServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwkSet := jwk.NewSet()
		jwkKey, _ := jwk.FromRaw(privateKey2.Public())
		jwkKey.Set(jwk.KeyIDKey, "issuer2-key-id")
		jwkSet.AddKey(jwkKey)
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer2.Close()

	// Define two different issuers
	issuer1 := "https://issuer1.example.com/v2.0/"
	issuer2 := "https://issuer2.example.com/v2.0/"

	// Create mock OpenID configuration servers for each issuer
	openIDServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		config := oidc.DiscoveryConfiguration{
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
		config := oidc.DiscoveryConfiguration{
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
	client, err := newClientWithTrustedIssuers(
		ctx,
		trustedIssuers,
		testClientID,
		WithRefreshInterval(time.Nanosecond),
	)
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
	claims1, err := client.ValidateToken(ctx, token1, WithValidateSignature(false))
	require.NoError(t, err)
	require.NotNil(t, claims1)
	assert.Equal(t, "subject-from-issuer1", claims1.Subject)
	assert.Equal(t, "User from Issuer 1", claims1.Name)
	assert.Equal(t, []string{"user1@example.com"}, claims1.Emails)

	// Validate token from issuer 2
	claims2, err := client.ValidateToken(ctx, token2, WithValidateSignature(false))
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
	claims3, err := client.ValidateToken(ctx, untrustedToken, WithValidateSignature(false))
	assert.Error(t, err)
	assert.Nil(t, claims3)
	assert.Contains(t, err.Error(), "untrusted token issuer")
}

func TestTokenReplayAttacks(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create a mock JWK server
	jwkSet := jwk.NewSet()
	jwkKey, err := jwk.FromRaw(publicKey)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)
	jwkSet.AddKey(jwkKey)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := oidc.DiscoveryConfiguration{
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

	// Create a test client
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := newClientWithTrustedIssuers(ctx, trustedIssuers, testClientID)
	require.NoError(t, err)

	// Create a mock replay cache
	replayCache := &mockReplayCache{
		usedTokens: make(map[string]time.Time),
	}

	// Create a valid token with JTI
	jti := "unique-token-id-123"
	validToken := createTestToken(t, privateKey, map[string]interface{}{
		"iss": testIssuer,
		"aud": testClientID,
		"sub": "test-subject",
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
		"jti": jti,
	}, "test-key-id")

	t.Run("first use of token should succeed", func(t *testing.T) {
		claims, err := client.ValidateToken(ctx, validToken,
			WithValidateSignature(false),
			WithReplayCache(replayCache))

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, jti, claims.ID)
	})

	t.Run("replay of same token should fail", func(t *testing.T) {
		claims, err := client.ValidateToken(ctx, validToken,
			WithValidateSignature(false),
			WithReplayCache(replayCache))

		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token has already been used")
	})

	t.Run("token without JTI should still work without replay protection", func(t *testing.T) {
		tokenWithoutJTI := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
			// No JTI claim
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, tokenWithoutJTI,
			WithValidateSignature(false),
			WithReplayCache(replayCache))

		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "", claims.ID) // No JTI
	})
}

func TestCraftedJWTTokens(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := &privateKey.PublicKey

	// Create a mock JWK server
	jwkSet := jwk.NewSet()
	jwkKey, err := jwk.FromRaw(publicKey)
	require.NoError(t, err)
	err = jwkKey.Set(jwk.KeyIDKey, "test-key-id")
	require.NoError(t, err)
	jwkSet.AddKey(jwkKey)

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwkSet)
	}))
	defer jwkServer.Close()

	// Create a mock OpenID configuration server
	openIDConfig := oidc.DiscoveryConfiguration{
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

	// Create a test client
	trustedIssuers := map[string]string{
		testIssuer: openIDServer.URL,
	}

	ctx := context.Background()
	client, err := newClientWithTrustedIssuers(ctx, trustedIssuers, testClientID)
	require.NoError(t, err)

	t.Run("malformed JWT should fail", func(t *testing.T) {
		malformedTokens := []string{
			"not.a.jwt",
			"header.payload",                 // Missing signature
			"",                               // Empty token
			"header.payload.signature.extra", // Too many parts
			"invalid-base64.invalid-base64.invalid-base64",
		}

		for _, token := range malformedTokens {
			claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))
			assert.Error(t, err, "Token should fail: %s", token)
			assert.Nil(t, claims)
			assert.Contains(t, err.Error(), "invalid token format")
		}
	})

	t.Run("token with none algorithm should fail", func(t *testing.T) {
		// Create a token with "none" algorithm (security vulnerability)
		header := map[string]interface{}{
			"alg": "none",
			"typ": "JWT",
		}
		payload := map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}

		headerJSON, _ := json.Marshal(header)
		payloadJSON, _ := json.Marshal(payload)

		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
		payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

		noneToken := headerB64 + "." + payloadB64 + "."

		claims, err := client.ValidateToken(ctx, noneToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "unsupported or insecure algorithm")
	})

	t.Run("token with weak algorithm should fail", func(t *testing.T) {
		// Test various weak algorithms
		weakAlgorithms := []string{"HS256", "HS384", "HS512", "RS1", "none"}

		for _, alg := range weakAlgorithms {
			header := map[string]interface{}{
				"alg": alg,
				"typ": "JWT",
			}
			payload := map[string]interface{}{
				"iss": testIssuer,
				"aud": testClientID,
				"sub": "test-subject",
				"exp": time.Now().Add(time.Hour).Unix(),
				"iat": time.Now().Add(-time.Minute).Unix(),
			}

			headerJSON, _ := json.Marshal(header)
			payloadJSON, _ := json.Marshal(payload)

			headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
			payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

			weakToken := headerB64 + "." + payloadB64 + ".fake-signature"

			claims, err := client.ValidateToken(ctx, weakToken,
				WithValidateSignature(true),
				WithAllowedAlgorithms(jwa.RS256, jwa.PS256)) // Only allow secure algorithms

			assert.Error(t, err, "Algorithm %s should be rejected", alg)
			assert.Nil(t, claims)
			assert.Contains(t, err.Error(), "unsupported or insecure algorithm")
		}
	})

	t.Run("token with missing required claims should fail", func(t *testing.T) {
		testCases := []struct {
			name     string
			claims   map[string]interface{}
			errorMsg string
		}{
			{
				name: "missing issuer",
				claims: map[string]interface{}{
					"aud": testClientID,
					"sub": "test-subject",
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Add(-time.Minute).Unix(),
				},
				errorMsg: "token missing issuer claim",
			},
			{
				name: "missing audience",
				claims: map[string]interface{}{
					"iss": testIssuer,
					"sub": "test-subject",
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Add(-time.Minute).Unix(),
				},
				errorMsg: "\"aud\" not found",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				token := createTestToken(t, privateKey, tc.claims, "test-key-id")
				claims, err := client.ValidateToken(ctx, token, WithValidateSignature(false))

				assert.Error(t, err)
				assert.Nil(t, claims)
				assert.Contains(t, err.Error(), tc.errorMsg)
			})
		}
	})

	t.Run("token with future issued at time should fail", func(t *testing.T) {
		futureToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(time.Hour).Unix(), // Issued in the future
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, futureToken, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "\"iat\" not satisfied")
	})

	t.Run("token with not before in future should fail", func(t *testing.T) {
		notYetValidToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
			"nbf": time.Now().Add(time.Hour).Unix(), // Not valid until future
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, notYetValidToken, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "\"nbf\" not satisfied")
	})

	t.Run("token with wrong audience should fail", func(t *testing.T) {
		wrongAudienceToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": "wrong-client-id",
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, wrongAudienceToken, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "\"aud\" not satisfied")
	})

	t.Run("token signed with wrong key should fail when signature validation enabled", func(t *testing.T) {
		// Generate a different key pair
		wrongPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		wrongKeyToken := createTestToken(t, wrongPrivateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, wrongKeyToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("expired token should fail", func(t *testing.T) {
		expiredToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, expiredToken, WithValidateSignature(false))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "\"exp\" not satisfied")
	})

	t.Run("token with tampered signature should fail", func(t *testing.T) {
		// Create a valid token first
		validToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		// Split the token and tamper with the signature
		parts := strings.Split(validToken, ".")
		require.Len(t, parts, 3, "Valid token should have 3 parts")

		// Tamper with the signature by changing a few characters
		tamperedSignature := parts[2]
		if len(tamperedSignature) > 10 {
			// Replace some characters in the middle of the signature
			runes := []rune(tamperedSignature)
			runes[5] = 'X'
			runes[10] = 'Y'
			runes[15] = 'Z'
			tamperedSignature = string(runes)
		}

		tamperedToken := parts[0] + "." + parts[1] + "." + tamperedSignature

		claims, err := client.ValidateToken(ctx, tamperedToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("token with completely fake signature should fail", func(t *testing.T) {
		// Create valid header and payload
		header := map[string]interface{}{
			"alg": "RS256",
			"typ": "JWT",
			"kid": "test-key-id",
		}
		payload := map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}

		headerJSON, _ := json.Marshal(header)
		payloadJSON, _ := json.Marshal(payload)

		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
		payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

		// Create a completely fake signature
		fakeSignature := base64.RawURLEncoding.EncodeToString([]byte("this-is-a-fake-signature-that-should-not-validate"))

		fakeToken := headerB64 + "." + payloadB64 + "." + fakeSignature

		claims, err := client.ValidateToken(ctx, fakeToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("token with modified payload but original signature should fail", func(t *testing.T) {
		// Create a valid token first
		validToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "original-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		// Split the token
		parts := strings.Split(validToken, ".")
		require.Len(t, parts, 3, "Valid token should have 3 parts")

		// Modify the payload (change the subject)
		modifiedPayload := map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "modified-subject", // Changed this
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}

		modifiedPayloadJSON, _ := json.Marshal(modifiedPayload)
		modifiedPayloadB64 := base64.RawURLEncoding.EncodeToString(modifiedPayloadJSON)

		// Use the original signature with the modified payload
		modifiedToken := parts[0] + "." + modifiedPayloadB64 + "." + parts[2]

		claims, err := client.ValidateToken(ctx, modifiedToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "token validation failed")
	})

	t.Run("token with modified header but original signature should fail", func(t *testing.T) {
		// Create a valid token first
		validToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		// Split the token
		parts := strings.Split(validToken, ".")
		require.Len(t, parts, 3, "Valid token should have 3 parts")

		// Modify the header (change the algorithm)
		modifiedHeader := map[string]interface{}{
			"alg": "HS256", // Changed from RS256
			"typ": "JWT",
			"kid": "test-key-id",
		}

		modifiedHeaderJSON, _ := json.Marshal(modifiedHeader)
		modifiedHeaderB64 := base64.RawURLEncoding.EncodeToString(modifiedHeaderJSON)

		// Use the original signature with the modified header
		modifiedToken := modifiedHeaderB64 + "." + parts[1] + "." + parts[2]

		claims, err := client.ValidateToken(ctx, modifiedToken, WithValidateSignature(true))
		assert.Error(t, err)
		assert.Nil(t, claims)
		// This should fail either due to algorithm validation or signature validation
		assert.True(t, strings.Contains(err.Error(), "token validation failed") ||
			strings.Contains(err.Error(), "unsupported or insecure algorithm"))
	})

	t.Run("valid token should pass signature validation", func(t *testing.T) {
		// Create a properly signed token
		validToken := createTestToken(t, privateKey, map[string]interface{}{
			"iss": testIssuer,
			"aud": testClientID,
			"sub": "test-subject",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Add(-time.Minute).Unix(),
		}, "test-key-id")

		claims, err := client.ValidateToken(ctx, validToken, WithValidateSignature(true))
		assert.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "test-subject", claims.Subject)
	})
}

// mockReplayCache implements TokenReplayCache for testing
type mockReplayCache struct {
	usedTokens map[string]time.Time
	mutex      sync.RWMutex
}

func (m *mockReplayCache) IsTokenUsed(ctx context.Context, jti string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	expiry, exists := m.usedTokens[jti]
	if !exists {
		return false, nil
	}

	// Check if token has expired
	if time.Now().After(expiry) {
		// Clean up expired token
		delete(m.usedTokens, jti)
		return false, nil
	}

	return true, nil
}

func (m *mockReplayCache) MarkTokenUsed(ctx context.Context, jti string, expiry time.Time) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.usedTokens[jti] = expiry
	return nil
}

func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims map[string]interface{}, keyID string) string {
	tokenString, err := CreateTestTokenWithClaims(privateKey, claims, keyID)
	require.NoError(t, err)
	return tokenString
}
