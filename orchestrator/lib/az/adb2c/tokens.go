package adb2c

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client represents an Azure ADB2C client for token validation
type Client struct {
	ClientID    string
	jwksFetcher *jwk.Cache

	// Map of trusted issuers to their discovery endpoints
	trustedIssuers     map[string]string
	issuerConfigs      map[string]*oidc.DiscoveryConfiguration
	issuerConfigsMutex sync.RWMutex

	// Last refresh time for OpenID configurations
	lastRefresh time.Time
	// How often to refresh the OpenID configurations (default: 24h)
	refreshInterval time.Duration
}

// TokenClaims represents the claims in a JWT token
type TokenClaims struct {
	Issuer    string   `json:"iss,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Audience  []string `json:"aud,omitempty"`
	Expiry    int64    `json:"exp,omitempty"`
	NotBefore int64    `json:"nbf,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	ID        string   `json:"jti,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	Name      string   `json:"name,omitempty"`
	Emails    []string `json:"emails,omitempty"`
}

// TokenReplayCache interface for preventing token replay attacks
type TokenReplayCache interface {
	// IsTokenUsed checks if a token with the given JTI has been used before
	IsTokenUsed(ctx context.Context, jti string) (bool, error)
	// MarkTokenUsed marks a token as used until its expiration time
	MarkTokenUsed(ctx context.Context, jti string, expiry time.Time) error
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithRefreshInterval sets the refresh interval for OpenID configurations
func WithRefreshInterval(interval time.Duration) ClientOption {
	return func(c *Client) {
		c.refreshInterval = interval
	}
}

// NewClient creates a new ADB2C client using a Config object.
// This is the preferred way to create a client when using koanf-based configuration.
func NewClient(ctx context.Context, config *Config, options ...ClientOption) (*Client, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if !config.Enabled {
		return nil, errors.New("ADB2C is not enabled in configuration")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	trustedIssuers := config.TrustedIssuersMap()
	return newClientWithTrustedIssuers(ctx, trustedIssuers, config.ADB2CClientID, options...)
}

// newClientWithTrustedIssuers creates a new ADB2C client using trusted issuers map
// This is an internal function used by NewClient and tests
func newClientWithTrustedIssuers(ctx context.Context, trustedIssuers map[string]string, clientID string, options ...ClientOption) (*Client, error) {
	if len(trustedIssuers) == 0 {
		return nil, errors.New("at least one trusted issuer is required")
	}

	client := &Client{
		ClientID:        clientID,
		trustedIssuers:  trustedIssuers,
		issuerConfigs:   make(map[string]*oidc.DiscoveryConfiguration),
		refreshInterval: 24 * time.Hour,
	}

	for _, option := range options {
		option(client)
	}

	// Initialize the jwk cache
	cache := jwk.NewCache(ctx)
	client.jwksFetcher = cache

	// Detect if all endpoints are direct JWKS URIs
	directJwksUris := true
	for _, endpoint := range trustedIssuers {
		if !strings.HasSuffix(endpoint, "/keys") &&
			!strings.HasSuffix(endpoint, "/jwks") &&
			!strings.HasSuffix(endpoint, "/jwks.json") &&
			!strings.Contains(endpoint, "/keys?p=") {
			directJwksUris = false
			break
		}
	}

	// If all endpoints are direct JWKS URIs, create minimal OpenID configurations
	if directJwksUris {
		for issuer, jwksURI := range trustedIssuers {
			config := &oidc.DiscoveryConfiguration{
				Issuer:  issuer,
				JwksURI: jwksURI,
			}
			client.issuerConfigs[issuer] = config

			// Register the JWKS URI with the cache for auto-refresh
			if err := cache.Register(jwksURI); err != nil {
				return nil, fmt.Errorf("failed to register JWKS URI %s: %w", jwksURI, err)
			}

			// Prefetch the JWKS
			if _, err := cache.Refresh(ctx, jwksURI); err != nil {
				return nil, fmt.Errorf("failed to refresh JWKS from %s: %w", jwksURI, err)
			}
		}
	} else {
		// Standard case - fetch OpenID configurations
		if err := client.refreshAllConfigurations(ctx); err != nil {
			return nil, fmt.Errorf("failed to initialize OpenID configurations: %w", err)
		}
	}

	return client, nil
}

// refreshAllConfigurations fetches OpenID configurations for all trusted issuers
func (c *Client) refreshAllConfigurations(ctx context.Context) error {
	c.issuerConfigsMutex.Lock()
	defer c.issuerConfigsMutex.Unlock()

	for issuer, discoveryURL := range c.trustedIssuers {
		// Check if the URL is a direct JWKS URI instead of a discovery endpoint
		// If it ends with '/keys' or '/jwks', assume it's a direct JWKS URI
		if strings.HasSuffix(discoveryURL, "/keys") || strings.HasSuffix(discoveryURL, "/jwks") {
			// Create a minimal OpenID configuration with just the JWKS URI
			config := &oidc.DiscoveryConfiguration{
				Issuer:  issuer,
				JwksURI: discoveryURL,
			}

			c.issuerConfigs[issuer] = config

			// Register the JWKS URI with the cache for auto-refresh
			if err := c.jwksFetcher.Register(discoveryURL); err != nil {
				return fmt.Errorf("failed to register JWKS URI: %w", err)
			}

			// Force a refresh of the JWKS to ensure it's loaded
			if _, err := c.jwksFetcher.Refresh(ctx, discoveryURL); err != nil {
				return fmt.Errorf("failed to refresh JWKS from %s: %w", discoveryURL, err)
			}

			continue
		}

		// Normal case - fetch OpenID configuration from discovery endpoint
		config, err := c.fetchOpenIDConfigurationFromURL(ctx, discoveryURL)
		if err != nil {
			return fmt.Errorf("failed to fetch OpenID configuration for issuer %s: %w", issuer, err)
		}

		// Verify the issuer in the config matches the expected issuer
		// Skip this check for direct JWKS URIs (already handled above)
		if config.Issuer != issuer && config.Issuer != "" {
			return fmt.Errorf("issuer mismatch: expected %s, got %s", issuer, config.Issuer)
		}

		// If empty, use the expected issuer
		if config.Issuer == "" {
			config.Issuer = issuer
		}

		c.issuerConfigs[issuer] = config

		if err := c.jwksFetcher.Register(config.JwksURI); err != nil {
			return fmt.Errorf("failed to register JWKS URI: %w", err)
		}

		// Force a refresh of the JWKS to ensure it's loaded and processed
		keySet, err := c.jwksFetcher.Refresh(ctx, config.JwksURI)
		if err != nil {
			return fmt.Errorf("failed to refresh JWKS from %s: %w", config.JwksURI, err)
		}

		c.ensureKeyAlgorithms(ctx, keySet)
	}

	c.lastRefresh = time.Now()
	return nil
}

// fetchOpenIDConfigurationFromURL fetches the OpenID configuration from a specific URL
func (c *Client) fetchOpenIDConfigurationFromURL(ctx context.Context, discoveryURL string) (*oidc.DiscoveryConfiguration, error) {
	// Create a context with timeout for security
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add security headers
	req.Header.Set("User-Agent", "ORCA-ADB2C-Client/1.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")

	// Use a secure HTTP client with timeouts
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenID configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch OpenID configuration: status code %d", resp.StatusCode)
	}

	// Limit response size to prevent DoS attacks
	limitedReader := &io.LimitedReader{R: resp.Body, N: 1024 * 1024} // 1MB limit

	var config oidc.DiscoveryConfiguration
	if err := json.NewDecoder(limitedReader).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse OpenID configuration: %w", err)
	}

	return &config, nil
}

// fetchKeySet fetches the JWK set for a given issuer, refreshing if needed
func (c *Client) fetchKeySet(ctx context.Context, issuer string) (jwk.Set, error) {
	c.issuerConfigsMutex.RLock()
	issuerConfig, ok := c.issuerConfigs[issuer]
	c.issuerConfigsMutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no configuration found for issuer: %s", issuer)
	}

	// Check if refresh is needed
	if time.Since(c.lastRefresh) >= c.refreshInterval {
		// Since we do not store the last refresh time per issuer/token, we refresh all configurations
		if err := c.refreshAllConfigurations(ctx); err != nil {
			return nil, fmt.Errorf("failed to refresh configurations: %w", err)
		}

		// Get the possibly updated config
		c.issuerConfigsMutex.RLock()
		issuerConfig = c.issuerConfigs[issuer]
		c.issuerConfigsMutex.RUnlock()
	}

	// Fetch JWK set
	keySet, err := c.jwksFetcher.Get(ctx, issuerConfig.JwksURI)
	if err != nil {
		// Try refreshing once more before failing
		keySet, err = c.jwksFetcher.Refresh(ctx, issuerConfig.JwksURI)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", issuerConfig.JwksURI, err)
		}
	}

	if keySet == nil || keySet.Len() == 0 {
		return nil, fmt.Errorf("no keys found in JWKS from %s", issuerConfig.JwksURI)
	}

	c.ensureKeyAlgorithms(ctx, keySet)

	return keySet, nil
}

// ValidateTokenOption is a function that configures token validation
type ValidateTokenOption func(*validateTokenOptions)

type validateTokenOptions struct {
	validateSignature bool
	replayCache       TokenReplayCache
	allowedAlgorithms []jwa.SignatureAlgorithm
}

// WithValidateSignature sets whether to validate the token signature
func WithValidateSignature(validate bool) ValidateTokenOption {
	return func(options *validateTokenOptions) {
		options.validateSignature = validate
	}
}

// WithReplayCache sets the replay cache for preventing token reuse
func WithReplayCache(cache TokenReplayCache) ValidateTokenOption {
	return func(options *validateTokenOptions) {
		options.replayCache = cache
	}
}

// WithAllowedAlgorithms sets the allowed signing algorithms (defaults to RS256, RS384, RS512)
func WithAllowedAlgorithms(algorithms ...jwa.SignatureAlgorithm) ValidateTokenOption {
	return func(options *validateTokenOptions) {
		options.allowedAlgorithms = algorithms
	}
}

// ValidateToken validates the provided JWT token
func (c *Client) ValidateToken(ctx context.Context, tokenString string, options ...ValidateTokenOption) (*TokenClaims, error) {
	// Set default options
	opts := &validateTokenOptions{
		validateSignature: true,                                                      // Default to validating signatures in production
		allowedAlgorithms: []jwa.SignatureAlgorithm{jwa.RS256, jwa.RS384, jwa.RS512}, // Secure defaults
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	// First parse token without validation to get issuer and check replay (if needed)
	parseOpts := []jwt.ParseOption{jwt.WithValidate(false), jwt.WithVerify(false)}
	parsedToken, err := jwt.Parse([]byte(tokenString), parseOpts...)
	if err != nil {
		return nil, fmt.Errorf("invalid token format: %w", err)
	}

	issuer := parsedToken.Issuer()
	if issuer == "" {
		return nil, fmt.Errorf("token missing issuer claim")
	}

	if _, trusted := c.trustedIssuers[issuer]; !trusted {
		return nil, fmt.Errorf("untrusted token issuer: %s", issuer)
	}

	// Check for token replay if cache is provided (before full validation)
	if opts.replayCache != nil {
		jti := parsedToken.JwtID()
		if jti != "" {
			used, err := opts.replayCache.IsTokenUsed(ctx, jti)
			if err != nil {
				return nil, fmt.Errorf("failed to check token replay: %w", err)
			}
			if used {
				return nil, fmt.Errorf("token has already been used (replay attack detected)")
			}
		}
	}

	// Validate algorithm if signature validation is enabled
	if opts.validateSignature && len(opts.allowedAlgorithms) > 0 {
		// Parse JWT header to extract algorithm
		parts := strings.Split(tokenString, ".")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid token format: missing header or payload")
		}

		// Decode header
		headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, fmt.Errorf("failed to decode token header: %w", err)
		}

		var header struct {
			Alg string `json:"alg"`
		}
		if err := json.Unmarshal(headerBytes, &header); err != nil {
			return nil, fmt.Errorf("failed to parse token header: %w", err)
		}

		if header.Alg == "" {
			return nil, fmt.Errorf("token missing algorithm in header")
		}

		// Validate algorithm
		alg := jwa.SignatureAlgorithm(header.Alg)
		allowed := false
		for _, allowedAlg := range opts.allowedAlgorithms {
			if alg == allowedAlg {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("unsupported or insecure algorithm: %s", alg)
		}
	}

	// Now validate the token with jwx built-in validation (handles exp, nbf, iat, iss, aud)
	validateOpts := []jwt.ParseOption{
		jwt.WithIssuer(issuer),
		jwt.WithAudience(c.ClientID),
		jwt.WithValidate(true), // Enables exp, nbf, iat validation
	}

	// Handle signature validation
	if !opts.validateSignature {
		validateOpts = append(validateOpts, jwt.WithVerify(false))
	} else {
		// Fetch JWK set for validation
		keySet, err := c.fetchKeySet(ctx, issuer)
		if err != nil {
			return nil, err
		}
		validateOpts = append(validateOpts, jwt.WithKeySet(keySet))
	}

	// Validate the token with all options - jwx handles all standard validations
	parsedToken, err = jwt.Parse([]byte(tokenString), validateOpts...)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Extract claims into our custom structure
	claims := &TokenClaims{
		Issuer:   parsedToken.Issuer(),
		Subject:  parsedToken.Subject(),
		IssuedAt: parsedToken.IssuedAt().Unix(),
		ID:       parsedToken.JwtID(),
	}

	// Handle expiry
	if !parsedToken.Expiration().IsZero() {
		claims.Expiry = parsedToken.Expiration().Unix()
	}

	// Handle not before
	if !parsedToken.NotBefore().IsZero() {
		claims.NotBefore = parsedToken.NotBefore().Unix()
	}

	// Extract audience
	if aud := parsedToken.Audience(); len(aud) > 0 {
		claims.Audience = aud
	}

	// Extract custom claims
	privateClaims := parsedToken.PrivateClaims()

	// Extract roles
	if roles, ok := privateClaims["roles"]; ok {
		if rolesSlice, ok := roles.([]interface{}); ok {
			for _, role := range rolesSlice {
				if roleStr, ok := role.(string); ok {
					claims.Roles = append(claims.Roles, roleStr)
				}
			}
		}
	}

	// Extract name
	if name, ok := privateClaims["name"]; ok {
		if nameStr, ok := name.(string); ok {
			claims.Name = nameStr
		}
	}

	// Extract emails
	if emails, ok := privateClaims["emails"]; ok {
		if emailsSlice, ok := emails.([]interface{}); ok {
			for _, email := range emailsSlice {
				if emailStr, ok := email.(string); ok {
					claims.Emails = append(claims.Emails, emailStr)
				}
			}
		}
	}

	// Mark token as used for replay protection (after successful validation)
	if opts.replayCache != nil && claims.ID != "" {
		var expiry time.Time
		if claims.Expiry > 0 {
			expiry = time.Unix(claims.Expiry, 0)
		} else {
			// If no expiry, use a reasonable default (1 hour)
			expiry = time.Now().Add(time.Hour)
		}

		if err := opts.replayCache.MarkTokenUsed(ctx, claims.ID, expiry); err != nil {
			return nil, fmt.Errorf("failed to mark token as used: %w", err)
		}
	}

	return claims, nil
}

// ensureKeyAlgorithms ensures that all keys in the JWKS have an algorithm defined
func (c *Client) ensureKeyAlgorithms(ctx context.Context, keySet jwk.Set) {
	if keySet == nil {
		return
	}

	for iter := keySet.Keys(ctx); iter.Next(ctx); {
		pair := iter.Pair()
		key := pair.Value.(jwk.Key)

		if key.Algorithm() == jwa.InvalidKeyAlgorithm("") {
			// The 'alg' field is optional for Azure ADB2C keys, but required by jwk, so we set a default
			// For RSA and EC key types, RS256 is a reasonable default
			if key.KeyType() == jwa.RSA || key.KeyType() == jwa.EC {
				key.Set(jwk.AlgorithmKey, jwa.RS256)
			}
		}
	}
}
