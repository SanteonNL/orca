package adb2c

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
)

// TrustedIssuer represents a single trusted issuer configuration
type TrustedIssuer struct {
	// IssuerURL is the issuer URL from the JWT token (e.g., https://tenant.b2clogin.com/tenant.onmicrosoft.com/v2.0/)
	IssuerURL string `koanf:"issuerurl"`
	// DiscoveryURL is the OpenID Connect discovery endpoint or direct JWKS URL
	DiscoveryURL string `koanf:"discoveryurl"`
}

type Config struct {
	Enabled bool `koanf:"enabled"`
	// ADB2CClientID is the name of the ADB2C client to use for authentication.
	ADB2CClientID string `koanf:"clientid"`
	// ADB2CTrustedIssuers is a map of friendly names to trusted issuer configurations.
	// The friendly names are used as environment variable suffixes.
	ADB2CTrustedIssuers map[string]TrustedIssuer `koanf:"trustedissuers"`
}

// TrustedIssuersMap converts the config format to the format expected by the ADB2C client
func (c Config) TrustedIssuersMap() map[string]string {
	result := make(map[string]string)
	for _, issuer := range c.ADB2CTrustedIssuers {
		result[issuer.IssuerURL] = issuer.DiscoveryURL
	}
	return result
}

// Validate validates the configuration for security and correctness
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // Disabled config is always valid
	}

	if c.ADB2CClientID == "" {
		return errors.New("ADB2C client ID is required when ADB2C is enabled")
	}

	if len(c.ADB2CTrustedIssuers) == 0 {
		return errors.New("at least one trusted issuer is required when ADB2C is enabled")
	}

	for name, issuer := range c.ADB2CTrustedIssuers {
		if name == "" {
			return errors.New("trusted issuer name cannot be empty")
		}

		if issuer.IssuerURL == "" {
			return fmt.Errorf("issuer URL cannot be empty for issuer '%s'", name)
		}

		if issuer.DiscoveryURL == "" {
			return fmt.Errorf("discovery URL cannot be empty for issuer '%s'", name)
		}

		// Validate issuer URL
		issuerURL, err := url.Parse(issuer.IssuerURL)
		if err != nil {
			return fmt.Errorf("invalid issuer URL for issuer '%s': %w", name, err)
		}

		if issuerURL.Scheme != "https" {
			return fmt.Errorf("issuer URL must use HTTPS for issuer '%s'", name)
		}

		// Additional security checks for issuer URL
		if issuerURL.Host == "" {
			return fmt.Errorf("issuer URL must have a valid host for issuer '%s'", name)
		}

		// Validate discovery URL
		discoveryURL, err := url.Parse(issuer.DiscoveryURL)
		if err != nil {
			return fmt.Errorf("invalid discovery URL for issuer '%s': %w", name, err)
		}

		if discoveryURL.Scheme != "https" {
			return fmt.Errorf("discovery URL must use HTTPS for issuer '%s'", name)
		}

		// Additional security checks for discovery URL
		if discoveryURL.Host == "" {
			return fmt.Errorf("discovery URL must have a valid host for issuer '%s'", name)
		}
	}

	return nil
}

// LoadConfig loads the ADB2C configuration from environment variables using koanf.
// Environment variables should be prefixed with ADB2C_ and use underscores for nested keys.
// For example:
//   - ADB2C_ENABLED=true
//   - ADB2C_CLIENTID=my-client-id
//   - ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL=https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/
//   - ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL=https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration
func LoadConfig() (*Config, error) {
	k := koanf.New(".")

	// Load environment variables with ADB2C_ prefix
	err := k.Load(env.Provider("ADB2C_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "ADB2C_")), "_", ".", -1)
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	config := DefaultConfig()
	err = k.Unmarshal("", &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// DefaultConfig returns the default ADB2C configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:             false,
		ADB2CClientID:       "",
		ADB2CTrustedIssuers: make(map[string]TrustedIssuer),
	}
}
