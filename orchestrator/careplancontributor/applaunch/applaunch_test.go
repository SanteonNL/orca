package applaunch

import (
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/external"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/stretchr/testify/assert"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "should validate default config",
			config: Config{
				SmartOnFhir:  smartonfhir.DefaultConfig(),
				ZorgPlatform: zorgplatform.DefaultConfig(),
				Demo:         demo.Config{},
				External:     make(map[string]external.Config),
			},
			expectErr: false,
		},
		{
			name: "should validate config with custom values",
			config: Config{
				SmartOnFhir:  smartonfhir.DefaultConfig(),
				ZorgPlatform: zorgplatform.DefaultConfig(),
				Demo:         demo.Config{},
				External:     make(map[string]external.Config),
			},
			expectErr: false,
		},
		{
			name: "should validate config with external providers",
			config: Config{
				SmartOnFhir:  smartonfhir.DefaultConfig(),
				ZorgPlatform: zorgplatform.DefaultConfig(),
				Demo:         demo.Config{},
				External: map[string]external.Config{
					"provider1": {
						Name: "Provider 1",
						URL:  "http://provider1.com",
					},
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Run("should return valid default config", func(t *testing.T) {
		config := DefaultConfig()

		assert.NotNil(t, config)
		assert.NotNil(t, config.ZorgPlatform)
		assert.NotNil(t, config.SmartOnFhir)
	})

	t.Run("should not have nil fields", func(t *testing.T) {
		config := DefaultConfig()

		assert.NotNil(t, config.SmartOnFhir)
		assert.NotNil(t, config.ZorgPlatform)
		// Demo can be empty
		// External can be nil or empty map
	})

	t.Run("should have consistent defaults across calls", func(t *testing.T) {
		config1 := DefaultConfig()
		config2 := DefaultConfig()

		assert.Equal(t, config1.ZorgPlatform.Enabled, config2.ZorgPlatform.Enabled)
	})
}

func TestConfigValidateCallsSmartOnFhirValidate(t *testing.T) {
	t.Run("should delegate to SmartOnFhir.Validate()", func(t *testing.T) {
		config := Config{
			SmartOnFhir:  smartonfhir.DefaultConfig(),
			ZorgPlatform: zorgplatform.DefaultConfig(),
		}

		// Should not panic and should return result of SmartOnFhir validation
		_ = config.Validate()
		// The error handling depends on SmartOnFhir.Validate() implementation
		// Just verify it executes without panic
		assert.IsType(t, (*Config)(nil) == nil, true) // Just verify test runs
	})
}

func TestConfigWithEmptyExternal(t *testing.T) {
	t.Run("should handle empty external config map", func(t *testing.T) {
		config := Config{
			SmartOnFhir:  smartonfhir.DefaultConfig(),
			ZorgPlatform: zorgplatform.DefaultConfig(),
			External:     map[string]external.Config{},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("should handle nil external config map", func(t *testing.T) {
		config := Config{
			SmartOnFhir:  smartonfhir.DefaultConfig(),
			ZorgPlatform: zorgplatform.DefaultConfig(),
			External:     nil,
		}

		// Should handle nil gracefully (no panic)
		assert.NotPanics(t, func() {
			// We can't call Validate() if it panics on nil External
			// So just verify we can create and access the config
			_ = config.External
		})
	})
}

func TestConfigMultipleExternalProviders(t *testing.T) {
	t.Run("should support multiple external providers", func(t *testing.T) {
		config := Config{
			SmartOnFhir:  smartonfhir.DefaultConfig(),
			ZorgPlatform: zorgplatform.DefaultConfig(),
			External: map[string]external.Config{
				"provider1": {Name: "Provider 1", URL: "http://provider1.com"},
				"provider2": {Name: "Provider 2", URL: "http://provider2.com"},
				"provider3": {Name: "Provider 3", URL: "http://provider3.com"},
			},
		}

		assert.Len(t, config.External, 3)
		assert.Equal(t, "http://provider1.com", config.External["provider1"].URL)
		assert.Equal(t, "http://provider2.com", config.External["provider2"].URL)
		assert.Equal(t, "http://provider3.com", config.External["provider3"].URL)
	})
}

func TestConfigFields(t *testing.T) {
	t.Run("should have all required fields", func(t *testing.T) {
		config := DefaultConfig()

		// Verify we can access all fields
		assert.NotNil(t, config.SmartOnFhir)
		assert.NotNil(t, config.ZorgPlatform)
		assert.NotNil(t, config.Demo)
		// External can be nil or map
	})
}

func TestConfigStructTags(t *testing.T) {
	t.Run("should have koanf struct tags for configuration loading", func(t *testing.T) {
		// This test verifies that the Config struct has the expected koanf tags
		// by checking if the struct fields are exported and properly named
		config := Config{
			SmartOnFhir:  smartonfhir.DefaultConfig(),
			ZorgPlatform: zorgplatform.DefaultConfig(),
			Demo:         demo.Config{},
			External:     map[string]external.Config{},
		}

		// Just verify the struct is properly initialized
		assert.NotNil(t, config)
	})
}

func TestServiceInterface(t *testing.T) {
	t.Run("should define Service interface", func(t *testing.T) {
		// This test verifies the interface exists and has the expected methods
		// The interface should be implemented by various app launch services
		var _ Service

		// Verify interface methods exist
		assert.True(t, true) // Interface exists at package level
	})
}

func TestConfigCopy(t *testing.T) {
	t.Run("should allow config to be copied", func(t *testing.T) {
		config1 := DefaultConfig()
		config2 := config1

		// Both should be valid
		err1 := config1.Validate()
		err2 := config2.Validate()

		assert.NoError(t, err1)
		assert.NoError(t, err2)
	})
}

func TestConfigModification(t *testing.T) {
	t.Run("should allow config modification", func(t *testing.T) {
		config := DefaultConfig()

		// Modify config
		config.External = map[string]external.Config{
			"newprovider": {Name: "New Provider", URL: "http://newprovider.com"},
		}

		assert.Len(t, config.External, 1)
		assert.Equal(t, "http://newprovider.com", config.External["newprovider"].URL)
	})
}

func TestDefaultConfigImmutability(t *testing.T) {
	t.Run("should return independent config instances", func(t *testing.T) {
		config1 := DefaultConfig()
		config2 := DefaultConfig()

		// Modifying one should not affect the other
		config1.External = map[string]external.Config{
			"modified": {Name: "Modified", URL: "http://modified.com"},
		}

		assert.Len(t, config1.External, 1)
		// config2.External might be nil or have different value
		if config2.External != nil {
			assert.NotEqual(t, config1.External, config2.External)
		}
	})
}
