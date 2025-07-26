package careplanservice

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		err := Config{}.Validate()
		require.NoError(t, err)
	})
	t.Run("ok", func(t *testing.T) {
		err := Config{Enabled: true}.Validate()
		require.NoError(t, err)
	})
}
