package adb2c

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client represents an Azure ADB2C client for token validation
type Client struct {
	Issuer       string
	ClientID     string
	JwksURI      string
	jwksFetcher  *jwk.Cache
	openIDConfig *oidc.DiscoveryConfiguration

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

// ValidateAudience checks if the token's audience claim contains the expected clientID
func (c *TokenClaims) ValidateAudience(clientID string) bool {
	if clientID == "" {
		return false
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
	if c.IssuedAt > 0 && time.Unix(c.IssuedAt, 0).After(now.Add(30*time.Second)) {
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
		issuerConfigs:   make(map[string]*oidc.DiscoveryConfiguration),
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

	if defaultConfig, ok := client.issuerConfigs[client.Issuer]; ok {
		client.openIDConfig = defaultConfig
		client.JwksURI = defaultConfig.JwksURI
	} else {
		return nil, fmt.Errorf("default issuer %s not found in trusted issuers", client.Issuer)
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
		if keySet, err := c.jwksFetcher.Refresh(ctx, config.JwksURI); err != nil {
			return fmt.Errorf("failed to refresh JWKS from %s: %w", config.JwksURI, err)
		} else {
			// Ensure keys have proper algorithm set
			c.ensureKeyAlgorithms(ctx, keySet)
		}
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
func (c *Client) fetchOpenIDConfigurationFromURL(ctx context.Context, discoveryURL string) (*oidc.DiscoveryConfiguration, error) {
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

	var config oidc.DiscoveryConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
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
}

// WithValidateSignature sets whether to validate the token signature
func WithValidateSignature(validate bool) ValidateTokenOption {
	return func(options *validateTokenOptions) {
		options.validateSignature = validate
	}
}

// ValidateToken validates the provided JWT token
func (c *Client) ValidateToken(ctx context.Context, tokenString string, options ...ValidateTokenOption) (*TokenClaims, error) {
	// Set default options
	opts := &validateTokenOptions{
		validateSignature: true, // Default to validating signatures in production
	}

	// Apply options
	for _, option := range options {
		option(opts)
	}

	// First parse token without validation to get issuer, to know which key set to use
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

	// Now validate the token with appropriate options
	validateOpts := []jwt.ParseOption{
		jwt.WithIssuer(issuer),
		jwt.WithValidate(true),
	}

	// Add audience validation if client ID is provided
	if c.ClientID != "" {
		validateOpts = append(validateOpts, jwt.WithAudience(c.ClientID))
	}

	// Handle signature validation based on options
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

	// Validate the token with all options
	parsedToken, err = jwt.Parse([]byte(tokenString), validateOpts...)
	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	// Extract claims
	claims := &TokenClaims{}

	claims.Issuer = parsedToken.Issuer()
	claims.Subject = parsedToken.Subject()
	claims.IssuedAt = parsedToken.IssuedAt().Unix()
	claims.ID = parsedToken.JwtID()

	// Handle expiry
	if !parsedToken.Expiration().IsZero() {
		claims.Expiry = parsedToken.Expiration().Unix()
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

	// Validate audience
	if !claims.ValidateAudience(c.ClientID) {
		return nil, fmt.Errorf("token audience validation failed: expected %s, got %v", c.ClientID, claims.Audience)
	}

	// Validate issued at time
	if err := claims.ValidateIssuedAt(); err != nil {
		return nil, err
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
