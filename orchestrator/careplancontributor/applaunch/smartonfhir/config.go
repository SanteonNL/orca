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
	URL       string `koanf:"url"`
	ClientID  string `koanf:"clientid"`
	OAuth2URL string `koanf:"oauth2url"`
	Tenant    string `koanf:"tenant"`
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
		if issuer.Tenant == "" {
			return fmt.Errorf("issuer %s tenant is required", key)
		}
		if !strings.HasPrefix(issuer.URL, "https://") && !strings.HasPrefix(issuer.URL, "http://") {
			return fmt.Errorf("issuer %s URL must start with http:// or https://", key)
		}
		if issuer.ClientID == "" {
			return fmt.Errorf("issuer %s clientid is required", key)
		}
		if issuer.OAuth2URL != "" && !strings.HasPrefix(issuer.OAuth2URL, "https://") && !strings.HasPrefix(issuer.OAuth2URL, "http://") {
			return fmt.Errorf("issuer %s oauth2url must start with http:// or https://", key)
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
