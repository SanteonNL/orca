package careplancontributor

import (
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
	FrontendConfig  FrontendConfig        `koanf:"frontend"`
	FHIR            coolfhir.ClientConfig `koanf:"fhir"`
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
