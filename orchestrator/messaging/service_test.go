package messaging

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEntity_FullName(t *testing.T) {
	t.Run("with prefix", func(t *testing.T) {
		e := Entity{Name: "my-topic", Prefix: true}
		require.Equal(t, "env.my-topic", e.FullName("env."))
	})
	t.Run("without prefix", func(t *testing.T) {
		e := Entity{Name: "my-topic", Prefix: false}
		require.Equal(t, "my-topic", e.FullName("env."))
	})
}

func TestNew_DefaultsToMemoryBroker(t *testing.T) {
	broker, err := New(Config{}, nil)
	require.NoError(t, err)
	require.NotNil(t, broker)
	_, ok := broker.(*MemoryBroker)
	require.True(t, ok)
}

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
		t.Skip("TODO: enable when Azure ServiceBus is required for robust operation")
		c := Config{}
		err := c.Validate(true)
		require.EqualError(t, err, "production-grade messaging configuration (Azure ServiceBus) is required in strict mode")
	})

}
