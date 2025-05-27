package adb2c

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		// Clear any existing environment variables
		os.Unsetenv("ADB2C_ENABLED")
		os.Unsetenv("ADB2C_CLIENTID")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
		os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")

		config, err := LoadConfig()
		require.NoError(t, err)

		expected := DefaultConfig()
		assert.Equal(t, expected.Enabled, config.Enabled)
		assert.Equal(t, expected.ADB2CClientID, config.ADB2CClientID)
		assert.Equal(t, expected.ADB2CTrustedIssuers, config.ADB2CTrustedIssuers)
	})

	t.Run("config from environment variables", func(t *testing.T) {
		// Set environment variables
		os.Setenv("ADB2C_ENABLED", "true")
		os.Setenv("ADB2C_CLIENTID", "test-client-id")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT2_ISSUERURL", "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/")
		os.Setenv("ADB2C_TRUSTEDISSUERS_TENANT2_DISCOVERYURL", "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/.well-known/openid_configuration")

		defer func() {
			os.Unsetenv("ADB2C_ENABLED")
			os.Unsetenv("ADB2C_CLIENTID")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_ISSUERURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT2_ISSUERURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_TENANT2_DISCOVERYURL")
		}()

		config, err := LoadConfig()
		require.NoError(t, err)

		assert.True(t, config.Enabled)
		assert.Equal(t, "test-client-id", config.ADB2CClientID)
		assert.Equal(t, map[string]TrustedIssuer{
			"tenant1": {
				IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
				DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			},
			"tenant2": {
				IssuerURL:    "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/",
				DiscoveryURL: "https://tenant2.b2clogin.com/tenant2.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			},
		}, config.ADB2CTrustedIssuers)
	})

	t.Run("partial config from environment", func(t *testing.T) {
		// Set only some environment variables
		os.Setenv("ADB2C_ENABLED", "true")
		os.Setenv("ADB2C_TRUSTEDISSUERS_MYISSUER_ISSUERURL", "https://myissuer.b2clogin.com/myissuer.onmicrosoft.com/v2.0/")
		os.Setenv("ADB2C_TRUSTEDISSUERS_MYISSUER_DISCOVERYURL", "https://myissuer.b2clogin.com/myissuer.onmicrosoft.com/v2.0/.well-known/openid_configuration")

		defer func() {
			os.Unsetenv("ADB2C_ENABLED")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_MYISSUER_ISSUERURL")
			os.Unsetenv("ADB2C_TRUSTEDISSUERS_MYISSUER_DISCOVERYURL")
		}()

		config, err := LoadConfig()
		require.NoError(t, err)

		assert.True(t, config.Enabled)
		assert.Equal(t, "", config.ADB2CClientID) // Should remain default
		assert.Equal(t, map[string]TrustedIssuer{
			"myissuer": {
				IssuerURL:    "https://myissuer.b2clogin.com/myissuer.onmicrosoft.com/v2.0/",
				DiscoveryURL: "https://myissuer.b2clogin.com/myissuer.onmicrosoft.com/v2.0/.well-known/openid_configuration",
			},
		}, config.ADB2CTrustedIssuers)
	})
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.False(t, config.Enabled)
	assert.Equal(t, "", config.ADB2CClientID)
	assert.NotNil(t, config.ADB2CTrustedIssuers)
	assert.Len(t, config.ADB2CTrustedIssuers, 0)
}

func TestConfig_ToTrustedIssuersMap(t *testing.T) {
	config := Config{
		ADB2CTrustedIssuers: map[string]TrustedIssuer{
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

	result := config.ToTrustedIssuersMap()

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
			Enabled:       true,
			ADB2CClientID: "", // Missing client ID
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:             true,
			ADB2CClientID:       "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{}, // Empty trusted issuers
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one trusted issuer is required")
	})

	t.Run("trusted issuer name cannot be empty", func(t *testing.T) {
		config := Config{
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
			Enabled:       true,
			ADB2CClientID: "test-client",
			ADB2CTrustedIssuers: map[string]TrustedIssuer{
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
