package main

import (
	"github.com/SanteonNL/orca/orchestrator/nuts"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"strings"
)

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
		},
	}
}

type Config struct {
	// Nuts holds the configuration for communicating with the Nuts API.
	Nuts nuts.Config `koanf:"nuts"`
	// Public holds the configuration for the public interface.
	Public InterfaceConfig `koanf:"public"`
	// URAMap holds a hardcoded map of URA identifiers to DIDs. It is later to be replaced with a more dynamic solution.
	// It's specified as followed:
	// ura1=did:example.com:bob,ura2=did:example.com:alice (etc)s
	URAMap map[string]string `koanf:"uramap"`
}

// InterfaceConfig holds the configuration for an HTTP interface.
type InterfaceConfig struct {
	// Address holds the address to listen on.
	Address string `koanf:"address"`
}
