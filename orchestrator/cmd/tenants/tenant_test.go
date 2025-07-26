package tenants

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("Nuts configuration", func(t *testing.T) {
		t.Run("subject not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID: "sub",
				},
			}
			err := c.Validate(false, false)
			require.EqualError(t, err, "tenant sub: missing Nuts subject")
		})
	})
	t.Run("ChipSoft configuration", func(t *testing.T) {
		t.Run("ChipSoftOrgID not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID:          "sub",
					NutsSubject: "subject",
				},
			}
			err := c.Validate(true, false)
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
			err := c.Validate(false, false)
			require.EqualError(t, err, "tenant sub: ChipSoftOrgID set, but Zorgplatform not enabled")
		})
	})
	t.Run("CarePlanService configuration", func(t *testing.T) {
		t.Run("CPSFHIR BaseURL not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID:          "sub",
					NutsSubject: "subject",
				},
			}
			err := c.Validate(false, true)
			require.EqualError(t, err, "tenant sub: CPS FHIR URL is not configured")
		})
	})
}
