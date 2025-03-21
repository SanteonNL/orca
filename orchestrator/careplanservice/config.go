package careplanservice

import (
	"errors"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

func DefaultConfig() Config {
	return Config{}
}

type Config struct {
	FHIR    coolfhir.ClientConfig `koanf:"fhir"`
	Enabled bool                  `koanf:"enabled"`
	// Defaults to false, should not be set true in Test or Prod
	AllowUnmanagedFHIROperations bool `koanf:"allowunmanagedfhiroperations"`
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
