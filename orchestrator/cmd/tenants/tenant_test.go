package tenants

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("subject not set", func(t *testing.T) {
		c := Config{
			"sub": Properties{
				ID: "sub",
			},
		}
		err := c.Validate(false)
		require.EqualError(t, err, "tenant sub: missing Nuts subject")
	})
	t.Run("ChipSoftOrgID not set", func(t *testing.T) {
		c := Config{
			"sub": Properties{
				ID:          "sub",
				NutsSubject: "subject",
			},
		}
		err := c.Validate(true)
		require.EqualError(t, err, "tenant sub: missing ChipSoftOrgID")
	})
	t.Run("ChipSoftOrgID set, but Zorgplatform not enabled", func(t *testing.T) {
		c := Config{
			"sub": Properties{
				ID:            "sub",
				NutsSubject:   "subject",
				ChipSoftOrgID: "test-org-id",
			},
		}
		err := c.Validate(false)
		require.EqualError(t, err, "tenant sub: ChipSoftOrgID set, but Zorgplatform not enabled")
	})
}
