package nuts

import (
	"errors"
	"net/url"
)

type Config struct {
	API              APIConfig           `koanf:"api"`
	Public           PublicConfig        `koanf:"public"`
	OwnSubject       string              `koanf:"subject"`
	DiscoveryService string              `koanf:"discoveryservice"`
	AzureKeyVault    AzureKeyVaultConfig `koanf:"azurekv"`
}

type AzureKeyVaultConfig struct {
	URL            string   `koanf:"url"`
	CredentialType string   `koanf:"credentialtype"`
	ClientCertName []string `koanf:"clientcertname"`
}

func (c Config) Validate() error {
	_, err := url.Parse(c.API.URL)
	if c.OwnSubject == "" {
		return errors.New("invalid/empty Nuts subject")
	}
	if err != nil || c.API.URL == "" {
		return errors.New("invalid Nuts API URL")
	}
	if c.Public.URL == "" {
		return errors.New("invalid/empty Nuts public URL")
	}
	if c.DiscoveryService == "" {
		return errors.New("invalid/empty Discovery Service ID")
	}
	if len(c.AzureKeyVault.ClientCertName) > 0 || c.AzureKeyVault.URL != "" {
		for _, clientCertName := range c.AzureKeyVault.ClientCertName {
			if clientCertName == "" {
				return errors.New("invalid/empty Azure Key Vault client certificate name")
			}
		}
		if c.AzureKeyVault.URL == "" {
			return errors.New("invalid/empty Azure Key Vault URL")
		}
	}
	return nil
}

type PublicConfig struct {
	URL string `koanf:"url"`
}

func (c PublicConfig) Parse() *url.URL {
	u, _ := url.Parse(c.URL)
	return u
}

type APIConfig struct {
	URL string `koanf:"url"`
}

func (n APIConfig) Parse() *url.URL {
	u, _ := url.Parse(n.URL)
	return u
}
