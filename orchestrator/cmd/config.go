package cmd

import (
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/rs/zerolog"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/knadh/koanf/v2"
	"net/url"
	"strings"

	"github.com/knadh/koanf/providers/env"
)

type Config struct {
	// Nuts holds the configuration for communicating with the Nuts API.
	Nuts nuts.Config `koanf:"nuts"`
	// Public holds the configuration for the public interface.
	Public InterfaceConfig `koanf:"public"`
	// CarePlanContributor holds the configuration for the CarePlanContributor.
	CarePlanContributor careplancontributor.Config `koanf:"careplancontributor"`
	// CarePlanService holds the configuration for the CarePlanService.
	CarePlanService careplanservice.Config `koanf:"careplanservice"`
	Tenants         tenants.Config         `koanf:"tenant"`
	Messaging       messaging.Config       `koanf:"messaging"`
	LogLevel        zerolog.Level          `koanf:"loglevel"`
	StrictMode      bool                   `koanf:"strictmode"`
	// OpenTelemetry holds the configuration for observability
	OpenTelemetry otel.Config `koanf:"opentelemetry"`
}

func (c Config) Validate() error {
	if err := c.Nuts.Validate(); err != nil {
		return fmt.Errorf("invalid Nuts configuration: %w", err)
	}
	if err := c.Tenants.Validate(c.CarePlanService.Enabled); err != nil {
		return fmt.Errorf("invalid tenant configuration: %w", err)
	}
	if err := c.Messaging.Validate(c.StrictMode); err != nil {
		return fmt.Errorf("invalid messaging configuration: %w", err)
	}
	if err := c.OpenTelemetry.Validate(); err != nil {
		return fmt.Errorf("invalid OpenTelemetry configuration: %w", err)
	}
	if c.Public.URL == "" {
		return errors.New("public base URL is not configured")
	}
	_, err := url.Parse(c.Public.URL)
	if err != nil {
		return errors.New("invalid public base URL")
	}
	if err := c.CarePlanContributor.Validate(); err != nil {
		return err
	}
	if err := c.CarePlanService.Validate(); err != nil {
		return err
	}
	return nil
}

// InterfaceConfig holds the configuration for an HTTP interface.
type InterfaceConfig struct {
	// Address holds the address to listen on.
	Address string `koanf:"address"`
	// URL holds the base URL of the interface.
	// Set it in case the service is behind a reverse proxy that maps it to a different URL than root (/).
	URL string `koanf:"url"`
}

func (i InterfaceConfig) ParseURL() *url.URL {
	u, _ := url.Parse(i.URL)
	return u
}

// LoadConfig loads the configuration from the environment.
func LoadConfig() (*Config, error) {
	result := DefaultConfig()
	err := loadConfigInto(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func loadConfigInto(target any) error {
	k := koanf.New(".")
	err := k.Load(env.ProviderWithValue("ORCA_", ".", func(key string, value string) (string, interface{}) {
		key = strings.Replace(strings.ToLower(strings.TrimPrefix(key, "ORCA_")), "_", ".", -1)
		if len(value) == 0 {
			return key, nil
		}
		sliceValues := splitWithEscaping(value, ",", "\\")
		for i, s := range sliceValues {
			sliceValues[i] = strings.TrimSpace(s)
		}
		var parsedValue any = sliceValues
		if len(sliceValues) == 1 {
			parsedValue = sliceValues[0]
		}
		return key, parsedValue
	}), nil)
	if err != nil {
		return err
	}
	return k.Unmarshal("", target)
}

func splitWithEscaping(s, separator, escape string) []string {
	s = strings.ReplaceAll(s, escape+separator, "\x00")
	tokens := strings.Split(s, separator)
	for i, token := range tokens {
		tokens[i] = strings.ReplaceAll(token, "\x00", separator)
	}
	return tokens
}

// DefaultConfig returns sensible, but not complete, default configuration values.
func DefaultConfig() Config {
	return Config{
		LogLevel:   zerolog.InfoLevel,
		StrictMode: true,
		Public: InterfaceConfig{
			Address: ":8080",
			URL:     "/",
		},
		CarePlanContributor: careplancontributor.DefaultConfig(),
		CarePlanService:     careplanservice.DefaultConfig(),
		OpenTelemetry:       otel.DefaultConfig(),
	}
}
