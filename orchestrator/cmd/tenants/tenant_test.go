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
			err := c.Validate(false)
			require.EqualError(t, err, "tenant sub: missing Nuts subject")
		})
	})
	t.Run("CarePlanService configuration", func(t *testing.T) {
		t.Run("CPSFHIR BaseURL not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID: "sub",
					Nuts: NutsProperties{
						Subject: "subject",
					},
				},
			}
			err := c.Validate(true)
			require.EqualError(t, err, "tenant sub: CPS FHIR URL is not configured")
		})
	})
}
