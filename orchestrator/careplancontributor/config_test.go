package careplancontributor

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		err := Config{}.Validate()
		require.NoError(t, err)
	})
	t.Run("missing care plan service URL", func(t *testing.T) {
		err := Config{
			Enabled: true,
		}.Validate()
		require.EqualError(t, err, "careplancontributor.careplanservice.url is not configured")
	})
}
