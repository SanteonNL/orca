package adb2c

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
)

// TestTokenGenerator provides utilities to create realistic Azure AD B2C test tokens
type TestTokenGenerator struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	KeyID      string
	TenantID   string
	ClientID   string
	Policy     string
	JWKSet     jwk.Set
}

// NewTestTokenGenerator creates a new test token generator with default settings
func NewTestTokenGenerator() (*TestTokenGenerator, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	keyID := "test-key-" + base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))[:8]

	// Create a JWK set with the public key
	jwkSet := jwk.NewSet()
	key, err := jwk.New(privateKey.Public())
	if err != nil {
		return nil, fmt.Errorf("failed to create JWK: %w", err)
	}

	err = key.Set(jwk.KeyIDKey, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to set key ID: %w", err)
	}

	jwkSet.Add(key)

	return &TestTokenGenerator{
		PrivateKey: privateKey,
		PublicKey:  &privateKey.PublicKey,
		KeyID:      keyID,
		TenantID:   "test-tenant",
		ClientID:   "test-client-id",
		Policy:     "B2C_1_test_policy",
		JWKSet:     jwkSet,
	}, nil
}

// GetIssuerURL returns the issuer URL for the token generator
func (g *TestTokenGenerator) GetIssuerURL() string {
	return fmt.Sprintf("https://%s.b2clogin.com/%s.onmicrosoft.com/v2.0/", g.TenantID, g.TenantID)
}

// CreateToken creates a realistic Azure AD B2C JWT token with the specified claims
func (g *TestTokenGenerator) CreateToken(customClaims map[string]interface{}) (string, error) {
	// Create standard claims
	now := time.Now()
	standardClaims := map[string]interface{}{
		"iss":                g.GetIssuerURL(),
		"aud":                g.ClientID,
		"sub":                "12345678-1234-1234-1234-123456789012",
		"exp":                now.Add(time.Hour).Unix(),
		"iat":                now.Unix(),
		"nbf":                now.Unix(),
		"auth_time":          now.Unix(),
		"ver":                "1.0",
		"name":               "Test User",
		"given_name":         "Test",
		"family_name":        "User",
		"emails":             []string{"test.user@example.com"},
		"tfp":                g.Policy,
		"scp":                "user_impersonation",
		"azp":                g.ClientID,
		"oid":                "87654321-4321-4321-4321-210987654321",
		"tid":                g.TenantID,
		"nonce":              "defaultNonce",
		"preferred_username": "test.user@example.com",
	}

	// Merge custom claims
	for k, v := range customClaims {
		standardClaims[k] = v
	}

	return CreateTestTokenWithClaims(g.PrivateKey, standardClaims, g.KeyID)
}

// CreateExpiredToken creates a token that is already expired
func (g *TestTokenGenerator) CreateExpiredToken() (string, error) {
	now := time.Now()
	return g.CreateToken(map[string]interface{}{
		"exp": now.Add(-time.Hour).Unix(),
		"iat": now.Add(-time.Hour * 2).Unix(),
	})
}

// CreateInvalidIssuerToken creates a token with an invalid issuer
func (g *TestTokenGenerator) CreateInvalidIssuerToken() (string, error) {
	return g.CreateToken(map[string]interface{}{
		"iss": "https://invalid-issuer.com/",
	})
}

// CreateInvalidAudienceToken creates a token with an invalid audience
func (g *TestTokenGenerator) CreateInvalidAudienceToken() (string, error) {
	return g.CreateToken(map[string]interface{}{
		"aud": "invalid-client-id",
	})
}

// CreateTokenWithRoles creates a token with the specified roles
func (g *TestTokenGenerator) CreateTokenWithRoles(roles []string) (string, error) {
	return g.CreateToken(map[string]interface{}{
		"roles": roles,
	})
}

// GetJWKSetJSON returns the JWK set as JSON
func (g *TestTokenGenerator) GetJWKSetJSON() ([]byte, error) {
	return json.Marshal(g.JWKSet)
}

// ExportPrivateKeyAsPEM exports the private key as a PEM string
func (g *TestTokenGenerator) ExportPrivateKeyAsPEM() string {
	privKeyBytes := x509.MarshalPKCS1PrivateKey(g.PrivateKey)
	privKeyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: privKeyBytes,
		},
	)
	return string(privKeyPEM)
}

// CreateTestTokenWithClaims creates a JWT token for testing with the specified RSA key, claims map, and key ID
func CreateTestTokenWithClaims(privateKey *rsa.PrivateKey, claims map[string]interface{}, keyID string) (string, error) {
	// Create the token
	token := jwt.New(jwt.SigningMethodRS256)
	token.Header["kid"] = keyID
	token.Header["typ"] = "JWT"
	token.Header["alg"] = "RS256"

	// Set claims
	mapClaims := token.Claims.(jwt.MapClaims)
	for k, v := range claims {
		mapClaims[k] = v
	}

	// Sign the token
	return token.SignedString(privateKey)
}
