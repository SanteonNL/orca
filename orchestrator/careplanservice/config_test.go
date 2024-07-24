package careplanservice

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		err := Config{}.Validate()
		require.NoError(t, err)
	})
	t.Run("FHIR server URL not configured", func(t *testing.T) {
		err := Config{Enabled: true}.Validate()
		require.EqualError(t, err, "careplanservice.fhir.url is not configured")
	})
	t.Run("ok", func(t *testing.T) {
		err := Config{Enabled: true, FHIR: FHIRConfig{BaseURL: "http://example.com"}}.Validate()
		require.NoError(t, err)
	})
}
