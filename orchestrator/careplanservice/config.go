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
	AllowUnmanagedFHIROperations bool   `koanf:"allowunmanagedfhiroperations"`
	AuditObserverSystem          string `koanf:"auditobserversystem"`
	AuditObserverValue           string `koanf:"auditobservervalue"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.FHIR.BaseURL == "" {
		return errors.New("careplanservice.fhir.url is not configured")
	}
	if c.AuditObserverSystem == "" {
		return errors.New("careplanservice.auditobserversystem is not configured")
	}
	if c.AuditObserverValue == "" {
		return errors.New("careplanservice.auditobservervalue is not configured")
	}
	return nil
}
