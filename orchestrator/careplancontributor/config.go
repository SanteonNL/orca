package careplancontributor

import "errors"

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
	FrontendConfig  FrontendConfig        `koanf:"frontend"`
	FHIR            FHIRConfig            `koanf:"fhir"`
	Enabled         bool                  `koanf:"enabled"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.CarePlanService.URL == "" {
		return errors.New("careplancontributor.careplanservice.url is not configured")
	}
	return nil
}

type CarePlanServiceConfig struct {
	// URL is the base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans.
	URL string `koanf:"url"`
}

type FrontendConfig struct {
	// URL is the base URL of the frontend for ORCA
	URL string `koanf:"url"`
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
