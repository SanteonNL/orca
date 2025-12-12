package oidc

import (
	"os"
	"strings"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/op"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Validate Provider config (should be zero value since OP doesn't have DefaultConfig)
	assert.False(t, config.Provider.Enabled)
	assert.Empty(t, config.Provider.Clients)

	// Validate RelyingParty config (should match rp.DefaultConfig())
	assert.False(t, config.RelyingParty.Enabled)
	assert.Equal(t, "", config.RelyingParty.ClientID)
	assert.NotNil(t, config.RelyingParty.TrustedIssuers)
	assert.Len(t, config.RelyingParty.TrustedIssuers, 0)
}

func TestConfig_Validate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := Config{
			Provider: op.Config{
				Enabled: true,
				Clients: map[string]op.ClientConfig{
					"test-client": {
						ID:          "test-client-id",
						RedirectURI: []string{"https://example.com/callback"},
						Secret:      "test-secret",
					},
				},
			},
			RelyingParty: rp.Config{
				Enabled:  true,
				ClientID: "test-rp-client",
				TrustedIssuers: map[string]rp.TrustedIssuer{
					"tenant1": {
						IssuerURL:    "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/",
						DiscoveryURL: "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration",
					},
				},
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("invalid relying party config", func(t *testing.T) {
		config := Config{
			Provider: op.Config{
				Enabled: false,
			},
			RelyingParty: rp.Config{
				Enabled:  true,
				ClientID: "", // Missing client ID should cause validation error
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client ID is required")
	})

	t.Run("disabled configs are valid", func(t *testing.T) {
		config := Config{
			Provider: op.Config{
				Enabled: false,
			},
			RelyingParty: rp.Config{
				Enabled: false,
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestConfig_KoanfIntegration(t *testing.T) {
	t.Run("loads config from environment variables", func(t *testing.T) {
		// Set up environment variables for both OP and RP configs
		os.Setenv("OIDC_PROVIDER_ENABLED", "true")
		os.Setenv("OIDC_PROVIDER_CLIENTS_CLIENT1_ID", "test-op-client-id")
		os.Setenv("OIDC_PROVIDER_CLIENTS_CLIENT1_REDIRECTURI", "https://example.com/callback,https://example.com/alt-callback")
		os.Setenv("OIDC_PROVIDER_CLIENTS_CLIENT1_SECRET", "test-secret")

		os.Setenv("OIDC_RELYINGPARTY_ENABLED", "true")
		os.Setenv("OIDC_RELYINGPARTY_CLIENTID", "test-rp-client")
		os.Setenv("OIDC_RELYINGPARTY_TRUSTEDISSUERS_TENANT1_ISSUERURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/")
		os.Setenv("OIDC_RELYINGPARTY_TRUSTEDISSUERS_TENANT1_DISCOVERYURL", "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration")

		defer func() {
			os.Unsetenv("OIDC_PROVIDER_ENABLED")
			os.Unsetenv("OIDC_PROVIDER_CLIENTS_CLIENT1_ID")
			os.Unsetenv("OIDC_PROVIDER_CLIENTS_CLIENT1_REDIRECTURI")
			os.Unsetenv("OIDC_PROVIDER_CLIENTS_CLIENT1_SECRET")
			os.Unsetenv("OIDC_RELYINGPARTY_ENABLED")
			os.Unsetenv("OIDC_RELYINGPARTY_CLIENTID")
			os.Unsetenv("OIDC_RELYINGPARTY_TRUSTEDISSUERS_TENANT1_ISSUERURL")
			os.Unsetenv("OIDC_RELYINGPARTY_TRUSTEDISSUERS_TENANT1_DISCOVERYURL")
		}()

		// Load configuration using koanf
		k := koanf.New(".")
		// Copied from orchestrator/cmd/config.go to mirror behaviour when the whole app is running
		err := k.Load(env.ProviderWithValue("OIDC_", ".", func(key string, value string) (string, interface{}) {
			key = strings.Replace(strings.ToLower(strings.TrimPrefix(key, "OIDC_")), "_", ".", -1)
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
		require.NoError(t, err)

		// Unmarshal into our config struct
		config := DefaultConfig()
		err = k.Unmarshal("", &config)
		require.NoError(t, err)

		// Verify Provider config
		assert.True(t, config.Provider.Enabled)
		assert.Len(t, config.Provider.Clients, 1)
		assert.Contains(t, config.Provider.Clients, "client1")

		client := config.Provider.Clients["client1"]
		assert.Equal(t, "test-op-client-id", client.ID)
		assert.Equal(t, []string{"https://example.com/callback", "https://example.com/alt-callback"}, client.RedirectURI)
		assert.Equal(t, "test-secret", client.Secret)

		// Verify RelyingParty config
		assert.True(t, config.RelyingParty.Enabled)
		assert.Equal(t, "test-rp-client", config.RelyingParty.ClientID)
		assert.Len(t, config.RelyingParty.TrustedIssuers, 1)
		assert.Contains(t, config.RelyingParty.TrustedIssuers, "tenant1")

		trustedIssuer := config.RelyingParty.TrustedIssuers["tenant1"]
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/", trustedIssuer.IssuerURL)
		assert.Equal(t, "https://tenant1.b2clogin.com/tenant1.onmicrosoft.com/v2.0/.well-known/openid_configuration", trustedIssuer.DiscoveryURL)

		// Verify validation passes
		err = config.Validate()
		assert.NoError(t, err)
	})
}

func splitWithEscaping(s, separator, escape string) []string {
	s = strings.ReplaceAll(s, escape+separator, "\x00")
	tokens := strings.Split(s, separator)
	for i, token := range tokens {
		tokens[i] = strings.ReplaceAll(token, "\x00", separator)
	}
	return tokens
}
