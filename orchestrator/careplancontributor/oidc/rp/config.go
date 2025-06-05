package rp

import (
	"errors"
	"fmt"
	"net/url"
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
	// ClientID is the name of the RelyingParty client to use for authentication.
	ClientID string `koanf:"clientid"`
	// TrustedIssuers is a map of friendly names to trusted issuer configurations.
	// The friendly names are used as environment variable suffixes.
	TrustedIssuers map[string]TrustedIssuer `koanf:"trustedissuers"`
}

// TrustedIssuersMap converts the config format to the format expected by the RelyingParty client
func (c Config) TrustedIssuersMap() map[string]string {
	result := make(map[string]string)
	for _, issuer := range c.TrustedIssuers {
		result[issuer.IssuerURL] = issuer.DiscoveryURL
	}
	return result
}

// Validate validates the configuration for security and correctness
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // Disabled config is always valid
	}

	if c.ClientID == "" {
		return errors.New("RelyingParty client ID is required when RelyingParty client is enabled")
	}

	if len(c.TrustedIssuers) == 0 {
		return errors.New("at least one trusted issuer is required when RelyingParty client is enabled")
	}

	for name, issuer := range c.TrustedIssuers {
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

// DefaultConfig returns the default RelyingParty configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		ClientID:       "",
		TrustedIssuers: make(map[string]TrustedIssuer),
	}
}
