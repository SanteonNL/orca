package smartonfhir

import (
	"errors"
	"fmt"
	"strings"
)

type Config struct {
	Enabled       bool                    `koanf:"enabled"`
	Issuer        map[string]IssuerConfig `koanf:"issuer"`
	AzureKeyVault AzureKeyVaultConfig     `koanf:"azurekv"`
}

type IssuerConfig struct {
	URL          string `koanf:"url"`
	ClientID     string `koanf:"clientid"`
	DiscoveryURL string `koanf:"discoveryurl"`
}

func DefaultConfig() Config {
	return Config{
		AzureKeyVault: AzureKeyVaultConfig{
			CredentialType: "managed_identity",
		},
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	for key, issuer := range c.Issuer {
		if !strings.HasPrefix(issuer.URL, "https://") && !strings.HasPrefix(issuer.URL, "http://") {
			return fmt.Errorf("issuer %s URL must start with http:// or https://", key)
		}
		if issuer.ClientID == "" {
			return fmt.Errorf("issuer %s clientid is required", key)
		}
	}
	return c.AzureKeyVault.Validate()
}

type AzureKeyVaultConfig struct {
	URL            string `koanf:"url"`
	CredentialType string `koanf:"credentialtype"`
	SigningKeyName string `koanf:"signingkey"`
}

func (c AzureKeyVaultConfig) Validate() error {
	if c.URL == "" {
		return nil
	}
	if c.CredentialType == "" {
		return errors.New("azurekv.credentialtype is required")
	}
	if c.SigningKeyName == "" {
		return errors.New("azurekv.signingkey is required")
	}
	return nil
}
