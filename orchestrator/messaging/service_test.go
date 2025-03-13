package messaging

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("strict mode with HTTP endpoint", func(t *testing.T) {
		c := Config{
			HTTP: HTTPBrokerConfig{
				Endpoint: "http://localhost:8080",
			},
		}
		err := c.Validate(true)
		require.Error(t, err)
	})
	t.Run("non-strictmode with HTTP endpoint", func(t *testing.T) {
		c := Config{
			HTTP: HTTPBrokerConfig{
				Endpoint: "http://localhost:8080",
			},
		}
		err := c.Validate(false)
		require.NoError(t, err)
	})
	t.Run("strict mode without Azure ServiceBus", func(t *testing.T) {
		c := Config{}
		err := c.Validate(true)
		require.EqualError(t, err, "production-grade messaging configuration (Azure ServiceBus) is required in strict mode")
	})

}
