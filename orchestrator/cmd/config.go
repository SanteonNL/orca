package cmd

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/lib/nuts"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/applaunch"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
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
	AppLaunch       applaunch.Config       `koanf:"applaunch"`
}

// InterfaceConfig holds the configuration for an HTTP interface.
type InterfaceConfig struct {
	// Address holds the address to listen on.
	Address string `koanf:"address"`
	// BaseURL holds the base URL of the interface.
	// Set it in case the service is behind a reverse proxy that maps it to a different URL than root (/).
	BaseURL string `koanf:"baseurl"`
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
			BaseURL: "/",
		},
	}
}
