package careplancontributor

import (
	"errors"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

func DefaultConfig() Config {
	return Config{}
}

type Config struct {
	CarePlanService   CarePlanServiceConfig `koanf:"careplanservice"`
	FrontendConfig    FrontendConfig        `koanf:"frontend"`
	AppLaunch         applaunch.Config      `koanf:"applaunch"`
	FHIR              coolfhir.ClientConfig `koanf:"fhir"`
	Enabled           bool                  `koanf:"enabled"`
	StaticBearerToken string
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
