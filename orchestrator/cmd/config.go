package cmd

import (
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	koanf "github.com/knadh/koanf/v2"
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
}

func (c Config) Validate() error {
	_, err := url.Parse(c.Nuts.API.URL)
	if c.Nuts.OwnSubject == "" {
		return errors.New("invalid/empty Nuts subject")
	}
	if err != nil || c.Nuts.API.URL == "" {
		return errors.New("invalid Nuts API URL")
	}
	if c.Nuts.Public.URL == "" {
		return errors.New("invalid/empty Nuts public URL")
	}
	_, err = url.Parse(c.Public.URL)
	if err != nil || c.Public.URL == "" {
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
	k := koanf.New(".")
	err := k.Load(env.Provider("ORCA_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "ORCA_")), "_", ".", -1)
	}), nil)
	if err != nil {
		return nil, err
	}

	result := DefaultConfig()
	if err := k.Unmarshal("", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DefaultConfig returns sensible, but not complete, default configuration values.
func DefaultConfig() Config {
	return Config{
		Public: InterfaceConfig{
			Address: ":8080",
			URL:     "/",
		},
		CarePlanContributor: careplancontributor.Config{
			Enabled: true,
		},
	}
}
