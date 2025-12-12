package careplanservice

func DefaultConfig() Config {
	return Config{}
}

type Config struct {
	Enabled bool         `koanf:"enabled"`
	Events  EventsConfig `koanf:"events"`
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
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
