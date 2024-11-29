package careplancontributor

import (
	"errors"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

func DefaultConfig() Config {
	return Config{
		Enabled:        true,
		AppLaunch:      applaunch.DefaultConfig(),
		SessionTimeout: 15 * time.Minute,
	}
}

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
	FrontendConfig  FrontendConfig        `koanf:"frontend"`
	AppLaunch       applaunch.Config      `koanf:"applaunch"`
	// FHIR contains the configuration to connect to the FHIR API holding EHR data,
	// to be made available through the CarePlanContributor.
	FHIR coolfhir.ClientConfig `koanf:"fhir"`
	// QuestionnaireFHIR contains the configuration to connect to the FHIR API holding Questionnaires and HealthcareServices,
	// used to negotiate FHIR Tasks. It might be a different FHIR API than the one holding EHR data,
	// also because HAPI doesn't allow storing Questionnaires in partitions.
	QuestionnaireFHIR             coolfhir.ClientConfig `koanf:"questionnairefhir"`
	Enabled                       bool                  `koanf:"enabled"`
	HealthDataViewEndpointEnabled bool                  `koanf:"healthdataviewendpointenabled"`
	SessionTimeout                time.Duration         `koanf:"sessiontimeout"`
	StaticBearerToken             string
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
