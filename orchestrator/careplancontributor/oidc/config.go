package oidc

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/op"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
)

// Config contains the configuration for both OpenID Connect Provider (OP) and Relying Party (RP)
type Config struct {
	// Provider contains the OpenID Connect Provider configuration
	Provider op.Config `koanf:"provider"`
	// RelyingParty contains the Relying Party configuration
	RelyingParty rp.Config `koanf:"relyingparty"`
}

// DefaultConfig returns the default OIDC configuration with both OP and RP defaults
func DefaultConfig() Config {
	return Config{
		Provider:     op.Config{},
		RelyingParty: rp.DefaultConfig(),
	}
}

// Validate validates both OP and RP configurations
func (c Config) Validate() error {
	// Validate RelyingParty configuration (OP doesn't have validation)
	if err := c.RelyingParty.Validate(); err != nil {
		return err
	}
	return nil
}
