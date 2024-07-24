package careplanservice

import "errors"

func DefaultConfig() Config {
	return Config{}
}

type Config struct {
	FHIR    FHIRConfig `koanf:"fhir"`
	Enabled bool       `koanf:"enabled"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.FHIR.BaseURL == "" {
		return errors.New("careplanservice.fhir.url is not configured")
	}
	return nil
}

type FHIRConfig struct {
	// BaseURL is the base URL of the FHIR server to connect to.
	BaseURL string `koanf:"url"`
	// Auth is the authentication configuration for the FHIR server.
	Auth FHIRAuthConfig `koanf:"auth"`
}

type FHIRAuthConfig struct {
	// Type of authentication to use, supported options: azure-managedidentity.
	// Leave empty for no authentication.
	Type string `koanf:"type"`
}
