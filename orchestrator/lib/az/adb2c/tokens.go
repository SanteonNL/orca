package adb2c

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
)

// Client represents an Azure ADB2C client for token validation
type Client struct {
	Issuer       string
	ClientID     string
	JwksURI      string
	jwksFetcher  *jwk.AutoRefresh
	openIDConfig *OpenIDConfig

	// Map of trusted issuers to their discovery endpoints
	trustedIssuers     map[string]string
	issuerConfigs      map[string]*OpenIDConfig
	issuerConfigsMutex sync.RWMutex

	// Last refresh time for OpenID configurations
	lastRefresh time.Time
	// How often to refresh the OpenID configurations (default: 24h)
	refreshInterval time.Duration
}

// OpenIDConfig represents the OpenID configuration for an ADB2C tenant
type OpenIDConfig struct {
	Issuer                 string   `json:"issuer"`
	AuthorizationEndpoint  string   `json:"authorization_endpoint"`
	TokenEndpoint          string   `json:"token_endpoint"`
	EndSessionEndpoint     string   `json:"end_session_endpoint"`
	JwksURI                string   `json:"jwks_uri"`
	ResponseModesSupported []string `json:"response_modes_supported"`
	ResponseTypesSupported []string `json:"response_types_supported"`
	ScopesSupported        []string `json:"scopes_supported"`
	SubjectTypesSupported  []string `json:"subject_types_supported"`
}

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	jwt.RegisteredClaims
	Roles  []string `json:"roles,omitempty"`
	Name   string   `json:"name,omitempty"`
	Emails []string `json:"emails,omitempty"`
}

// ValidateAudience checks if the token's audience claim contains the expected clientID
func (c *TokenClaims) ValidateAudience(clientID string) bool {
	if clientID == "" {
		return true
	}

	for _, aud := range c.Audience {
		if aud == clientID {
			return true
		}
	}
	return false
}

// ValidateIssuedAt checks if the token was issued at a reasonable time
func (c *TokenClaims) ValidateIssuedAt() error {
	now := time.Now()
	if c.IssuedAt != nil && c.IssuedAt.After(now.Add(30*time.Second)) {
		return errors.New("token was issued in the future")
	}
	return nil
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithRefreshInterval sets the refresh interval for OpenID configurations
func WithRefreshInterval(interval time.Duration) ClientOption {
	return func(c *Client) {
		c.refreshInterval = interval
	}
}

// WithDefaultIssuer sets the default issuer for the client
func WithDefaultIssuer(issuer string) ClientOption {
	return func(c *Client) {
		c.Issuer = issuer
	}
}

// NewClient creates a new ADB2C client using trusted issuers
// For the sake of config centralisation, the trusted issuers are passed as a map rather than read from ENV/config
func NewClient(ctx context.Context, trustedIssuers map[string]string, clientID string, options ...ClientOption) (*Client, error) {
	if len(trustedIssuers) == 0 {
		return nil, errors.New("at least one trusted issuer is required")
	}

	client := &Client{
		ClientID:        clientID,
		trustedIssuers:  trustedIssuers,
		issuerConfigs:   make(map[string]*OpenIDConfig),
		refreshInterval: 24 * time.Hour,
	}

	for _, option := range options {
		option(client)
	}

	// If no default issuer was provided, use the first one from the trusted issuers
	if client.Issuer == "" {
		for issuer := range trustedIssuers {
			client.Issuer = issuer
			break
		}
	}

	// Initialize the jwks fetcher with auto-refresh capability
	client.jwksFetcher = jwk.NewAutoRefresh(ctx)

	// Prefetch all OpenID configurations
	if err := client.refreshAllConfigurations(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenID configurations: %w", err)
	}

	return client, nil
}

// refreshAllConfigurations fetches OpenID configurations for all trusted issuers
func (c *Client) refreshAllConfigurations(ctx context.Context) error {
	c.issuerConfigsMutex.Lock()
	defer c.issuerConfigsMutex.Unlock()

	for issuer, discoveryURL := range c.trustedIssuers {
		config, err := c.fetchOpenIDConfigurationFromURL(ctx, discoveryURL)
		if err != nil {
			return fmt.Errorf("failed to fetch OpenID configuration for issuer %s: %w", issuer, err)
		}

		// Verify the issuer in the config matches the expected issuer
		if config.Issuer != issuer {
			return fmt.Errorf("issuer mismatch: expected %s, got %s", issuer, config.Issuer)
		}

		c.issuerConfigs[issuer] = config

		// Configure JWKS fetcher for this issuer
		c.jwksFetcher.Configure(config.JwksURI)
	}

	// Set the default OpenID config and JWKS URI
	if defaultConfig, ok := c.issuerConfigs[c.Issuer]; ok {
		c.openIDConfig = defaultConfig
		c.JwksURI = defaultConfig.JwksURI
	} else {
		return fmt.Errorf("default issuer %s not found in trusted issuers", c.Issuer)
	}

	c.lastRefresh = time.Now()
	return nil
}

// fetchOpenIDConfigurationFromURL fetches the OpenID configuration from a specific URL
func (c *Client) fetchOpenIDConfigurationFromURL(ctx context.Context, discoveryURL string) (*OpenIDConfig, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenID configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch OpenID configuration: status code %d", resp.StatusCode)
	}

	var config OpenIDConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse OpenID configuration: %w", err)
	}

	return &config, nil
}

// refreshConfigurationsIfNeeded refreshes OpenID configurations if the refresh interval has passed
func (c *Client) refreshConfigurationsIfNeeded(ctx context.Context) error {
	if time.Since(c.lastRefresh) < c.refreshInterval {
		return nil
	}

	return c.refreshAllConfigurations(ctx)
}

// ParseToken parses a JWT token without validation and returns the claims
func ParseToken(tokenString string) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format: token should have three parts")
	}

	// Decode the payload (second part)
	payload, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error decoding token payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("error parsing token claims: %w", err)
	}

	return claims, nil
}

// decodeSegment decodes a base64url encoded segment
func decodeSegment(seg string) ([]byte, error) {
	seg = strings.Map(func(r rune) rune {
		switch r {
		case '-':
			return '+'
		case '_':
			return '/'
		default:
			return r
		}
	}, seg)

	// Add padding if necessary
	if mod := len(seg) % 4; mod != 0 {
		seg += strings.Repeat("=", 4-mod)
	}

	return base64.StdEncoding.DecodeString(seg)
}

// ValidateToken validates the provided JWT token
func (c *Client) ValidateToken(ctx context.Context, tokenString string) (*TokenClaims, error) {
	// Refresh configurations if needed
	if err := c.refreshConfigurationsIfNeeded(ctx); err != nil {
		return nil, err
	}

	// Parse token without validating signature first to get claims and kid
	token, _ := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		return nil, nil
	})

	if token == nil {
		return nil, errors.New("invalid token format")
	}

	// Extract claims to check the issuer
	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Check if the issuer is trusted
	if _, trusted := c.trustedIssuers[claims.Issuer]; !trusted {
		return nil, fmt.Errorf("untrusted token issuer: %s", claims.Issuer)
	}

	// Get the issuer's OpenID configuration
	c.issuerConfigsMutex.RLock()
	config, ok := c.issuerConfigs[claims.Issuer]
	c.issuerConfigsMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no configuration found for issuer: %s", claims.Issuer)
	}

	// Get the key ID from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("token header missing 'kid'")
	}

	// Get the RSA public key for the token
	rsaKey, err := c.getPublicKeyFromJWKS(ctx, config.JwksURI, kid, claims.Issuer)
	if err != nil {
		return nil, err
	}

	// Parse and validate the token with the correct key
	validatedToken, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure correct signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return rsaKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token has expired")
		}
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	if !validatedToken.Valid {
		return nil, errors.New("token is invalid")
	}

	validatedClaims, ok := validatedToken.Claims.(*TokenClaims)
	if !ok {
		return nil, errors.New("failed to extract token claims")
	}

	if !validatedClaims.ValidateAudience(c.ClientID) {
		return nil, errors.New("invalid token audience")
	}

	if err := validatedClaims.ValidateIssuedAt(); err != nil {
		return nil, err
	}

	return validatedClaims, nil
}

// getPublicKeyFromJWKS fetches the public key corresponding to the key ID from the JWKS endpoint
func (c *Client) getPublicKeyFromJWKS(ctx context.Context, jwksURI, kid, issuer string) (*rsa.PublicKey, error) {
	jwks, err := c.jwksFetcher.Fetch(ctx, jwksURI)
	if err != nil {
		// If fetching fails, try to refresh the configuration once, this can happen if the keys have been rotated
		c.issuerConfigsMutex.Lock()
		defer c.issuerConfigsMutex.Unlock()

		refreshedConfig, refreshErr := c.fetchOpenIDConfigurationFromURL(ctx, c.trustedIssuers[issuer])
		if refreshErr == nil {
			c.issuerConfigs[issuer] = refreshedConfig
			// Update the JWKS URI
			c.jwksFetcher.Configure(refreshedConfig.JwksURI)
			jwks, err = c.jwksFetcher.Fetch(ctx, refreshedConfig.JwksURI)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
		}
	}

	// Find the key with the matching kid
	matchingKey, found := jwks.LookupKeyID(kid)
	if !found {
		return nil, errors.New("matching key not found in JWKS")
	}

	var rawKey interface{}
	if err := matchingKey.Raw(&rawKey); err != nil {
		return nil, fmt.Errorf("failed to get raw JWK: %w", err)
	}

	rsaKey, ok := rawKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	return rsaKey, nil
}

// ParseTrustedIssuers parses a string of trusted issuers in the format "issuer1=url1;issuer2=url2"
// into a map of issuer URLs to their corresponding discovery endpoints
func ParseTrustedIssuers(issuersStr string) (map[string]string, error) {
	if issuersStr == "" {
		return nil, fmt.Errorf("empty trusted issuers string")
	}

	trustedIssuers := make(map[string]string)
	pairs := strings.Split(issuersStr, ";")

	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid issuer mapping format: %s", pair)
		}

		issuer := strings.TrimSpace(parts[0])
		discoveryURL := strings.TrimSpace(parts[1])

		if issuer == "" || discoveryURL == "" {
			return nil, fmt.Errorf("empty issuer or discovery URL in mapping: %s", pair)
		}

		trustedIssuers[issuer] = discoveryURL
	}

	if len(trustedIssuers) == 0 {
		return nil, fmt.Errorf("no valid issuer mappings found")
	}

	return trustedIssuers, nil
}
