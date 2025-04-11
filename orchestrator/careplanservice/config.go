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
	Events  EventsConfig          `koanf:"events"`
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

type EventsConfig struct {
	WebHooks []WebHookEventHandlerConfig `koanf:"webhooks"`
}

type WebHookEventHandlerConfig struct {
	// URL is the URL to which the event should be sent.
	URL string
}
