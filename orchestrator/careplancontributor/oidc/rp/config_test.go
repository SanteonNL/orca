package rp

import (
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, "", config.ClientID)
	assert.NotNil(t, config.TrustedIssuers)
	assert.Len(t, config.TrustedIssuers, 0)
}

// TestConfig holds the configuration for the Azure AD B2C integration test
type TestConfig struct {
	Token    string
	ClientID string
	Config   *Config
}

func LoadTestConfig() *TestConfig {
	testConfig := &TestConfig{
		Token:    os.Getenv("ADB2C_TOKEN"),
		ClientID: os.Getenv("ADB2C_CLIENT_ID"),
	}

	// Load the koanf-based configuration directly
	k := koanf.New(".")

	// Load environment variables with ADB2C_ prefix
	err := k.Load(env.Provider("ADB2C_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(strings.TrimPrefix(s, "ADB2C_")), "_", ".", -1)
	}), nil)

	config := DefaultConfig()
	if err == nil {
		// If koanf loading succeeds, unmarshal into config
		k.Unmarshal("", &config)
	}

	// Override client ID from environment if provided
	if testConfig.ClientID != "" {
		config.ClientID = testConfig.ClientID
	}

	testConfig.Config = &config
	return testConfig
}

// TestLoadTestConfig tests the configuration loading for integration tests
func TestLoadTestConfig(t *testing.T) {
	t.Run("loads config with koanf environment variables", func(t *testing.T) {
		// Set up environment variables in the new koanf format
		os.Setenv("ADB2C_TOKEN", "test-token")
		os.Setenv("ADB2C_CLIENT_ID", "test-client-from-env")
		os.Setenv("ADB2C_ENABLED", "true")
		os.Setenv("ADB2C_CLIENTID", "test-client-from-koanf")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration")

		defer func() {
			os.Unsetenv("ADB2C_TOKEN")
			os.Unsetenv("ADB2C_CLIENT_ID")
			os.Unsetenv("ADB2C_ENABLED")
			os.Unsetenv("ADB2C_CLIENTID")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")
		}()

		testConfig := LoadTestConfig()

		// Verify token and client ID are loaded correctly
		assert.Equal(t, "test-token", testConfig.Token)
		assert.Equal(t, "test-client-from-env", testConfig.ClientID) // ADB2C_CLIENT_ID takes precedence

		// Verify koanf config is loaded
		assert.NotNil(t, testConfig.Config)
		assert.True(t, testConfig.Config.Enabled)
		assert.Equal(t, "test-client-from-env", testConfig.Config.ClientID) // Should be overridden by ADB2C_CLIENT_ID

		// Verify trusted issuers are loaded
		assert.Len(t, testConfig.Config.TrustedIssuers, 1)
		assert.Contains(t, testConfig.Config.TrustedIssuers, "tenant1")

		trustedIssuer := testConfig.Config.TrustedIssuers["tenant1"]
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/", trustedIssuer.IssuerURL)
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration", trustedIssuer.DiscoveryURL)

		// Verify TrustedIssuersMap works
		trustedIssuersMap := testConfig.Config.TrustedIssuersMap()
		assert.Len(t, trustedIssuersMap, 1)
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			trustedIssuersMap["https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/"])
	})

	t.Run("handles missing koanf config gracefully", func(t *testing.T) {
		// Clear all ADB2C environment variables
		os.Unsetenv("ADB2C_ENABLED")
		os.Unsetenv("ADB2C_CLIENTID")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")

		// Set only the test-specific variables
		os.Setenv("ADB2C_TOKEN", "test-token")
		os.Setenv("ADB2C_CLIENT_ID", "test-client")

		defer func() {
			os.Unsetenv("ADB2C_TOKEN")
			os.Unsetenv("ADB2C_CLIENT_ID")
		}()

		testConfig := LoadTestConfig()

		// Verify basic config is loaded
		assert.Equal(t, "test-token", testConfig.Token)
		assert.Equal(t, "test-client", testConfig.ClientID)

		// Verify fallback config is created
		assert.NotNil(t, testConfig.Config)
		assert.False(t, testConfig.Config.Enabled) // Should be false when no koanf config
		assert.Equal(t, "test-client", testConfig.Config.ClientID)
		assert.Len(t, testConfig.Config.TrustedIssuers, 0)
	})

	t.Run("client ID precedence", func(t *testing.T) {
		// Test that ADB2C_CLIENT_ID takes precedence over ADB2C_CLIENTID
		os.Setenv("ADB2C_CLIENT_ID", "client-from-client-id")
		os.Setenv("ADB2C_CLIENTID", "client-from-client")

		defer func() {
			os.Unsetenv("ADB2C_CLIENT_ID")
			os.Unsetenv("ADB2C_CLIENTID")
		}()

		testConfig := LoadTestConfig()

		assert.Equal(t, "client-from-client-id", testConfig.ClientID)
		assert.Equal(t, "client-from-client-id", testConfig.Config.ClientID)
	})
}

func TestConfig_ToTrustedIssuersMap(t *testing.T) {
	config := Config{
		TrustedIssuers: map[string]TrustedIssuer{
			"tenant1": {
				IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
				DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			},
			"tenant2": {
				IssuerURL:    "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/",
				DiscoveryURL: "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			},
		},
	}

	result := config.TrustedIssuersMap()

	expected := map[string]string{
		"https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/": "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
		"https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/": "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/.well-known/openid_configuration",
	}

	assert.Equal(t, expected, result)
}

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled config is always valid", func(t *testing.T) {
		config := Config{
			Enabled: false,
			// Other fields can be empty/invalid when disabled
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("enabled config requires client ID", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "", // Missing client ID
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ADB2C client ID is required")
	})

	t.Run("enabled config requires trusted issuers", func(t *testing.T) {
		config := Config{
			Enabled:        true,
			ClientID:       "test-client",
			TrustedIssuers: map[string]TrustedIssuer{}, // Empty trusted issuers
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one trusted issuer is required")
	})

	t.Run("trusted issuer name cannot be empty", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"": { // Empty issuer name
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "trusted issuer name cannot be empty")
	})

	t.Run("trusted issuer URL cannot be empty", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "", // Empty issuer URL
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issuer URL cannot be empty")
	})

	t.Run("trusted discovery URL cannot be empty", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "", // Empty discovery URL
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "discovery URL cannot be empty")
	})

	t.Run("trusted issuer URL must be valid", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "://invalid-url-with-no-scheme", // Invalid issuer URL
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid issuer URL")
	})

	t.Run("trusted discovery URL must be valid", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "://invalid-url-with-no-scheme", // Invalid discovery URL
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid discovery URL")
	})

	t.Run("trusted issuer URL must use HTTPS", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "http://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/", // HTTP instead of HTTPS
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issuer URL must use HTTPS")
	})

	t.Run("trusted discovery URL must use HTTPS", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "http://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration", // HTTP instead of HTTPS
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "discovery URL must use HTTPS")
	})

	t.Run("valid enabled config", func(t *testing.T) {
		config := Config{
			Enabled:  true,
			ClientID: "test-client",
			TrustedIssuers: map[string]TrustedIssuer{
				"tenant1": {
					IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
				"tenant2": {
					IssuerURL:    "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/",
					DiscoveryURL: "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/.well-known/openid_configuration",
				},
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})
}
