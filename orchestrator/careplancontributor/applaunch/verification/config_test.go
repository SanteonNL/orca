package verification

import (
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	issuers := map[string]rp.TrustedIssuer{
		"entra": {IssuerURL: "https://login.microsoftonline.com/tenant/v2.0", DiscoveryURL: "https://login.microsoftonline.com/tenant/v2.0/.well-known/openid-configuration"},
	}

	t.Run("disabled needs no OIDC", func(t *testing.T) {
		require.NoError(t, Config{}.Validate())
	})
	t.Run("enabled without OIDC is valid: the node only accepts delegated launches", func(t *testing.T) {
		require.NoError(t, Config{Enabled: true}.Validate())
	})
	t.Run("OIDC without audience is rejected", func(t *testing.T) {
		err := Config{Enabled: true, OIDC: rp.Config{Enabled: true, TrustedIssuers: issuers}}.Validate()
		assert.ErrorContains(t, err, "clientid")
	})
	t.Run("OIDC without trusted issuers is rejected", func(t *testing.T) {
		err := Config{Enabled: true, OIDC: rp.Config{Enabled: true, ClientID: "audience"}}.Validate()
		assert.ErrorContains(t, err, "trusted OIDC issuer")
	})
	t.Run("fully configured OIDC is valid", func(t *testing.T) {
		require.NoError(t, Config{Enabled: true, OIDC: rp.Config{Enabled: true, ClientID: "audience", TrustedIssuers: issuers}}.Validate())
	})
}
