package verification

import (
	"errors"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
)

type Config struct {
	Enabled bool `koanf:"enabled"`
	// OIDC configures which identity provider's tokens may initiate a verification launch
	// (e.g. Entra ID: issuer https://login.microsoftonline.com/{tenant}/v2.0, audience = app registration).
	// It is only needed on the node a workforce application calls directly; launches delegated by
	// another SCP node are authenticated through Shared Care Planning itself.
	OIDC rp.Config `koanf:"oidc"`
}

func (c Config) Validate() error {
	if !c.Enabled || !c.OIDC.Enabled {
		return nil
	}
	if c.OIDC.ClientID == "" {
		return errors.New("verification app launch: OIDC audience (clientid) is required")
	}
	if len(c.OIDC.TrustedIssuers) == 0 {
		return errors.New("verification app launch: at least one trusted OIDC issuer is required")
	}
	return nil
}
