package nuts

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("discovery service ID not set", func(t *testing.T) {
		c := Config{
			API: APIConfig{
				URL: "http://nutsnode:8081",
			},
			Public: PublicConfig{
				URL: "http://nutsnode:8080",
			},
			OwnSubject: "sub",
		}
		err := c.Validate()
		require.EqualError(t, err, "invalid/empty Discovery Service ID")
	})
	t.Run("public URL not set", func(t *testing.T) {
		c := Config{
			API: APIConfig{
				URL: "http://nutsnode:8081",
			},
			DiscoveryService: "discovery",
			OwnSubject:       "sub",
		}
		err := c.Validate()
		require.EqualError(t, err, "invalid/empty Nuts public URL")
	})
	t.Run("API URL not set", func(t *testing.T) {
		c := Config{
			Public: PublicConfig{
				URL: "http://nutsnode:8080",
			},
			DiscoveryService: "discovery",
			OwnSubject:       "sub",
		}
		err := c.Validate()
		require.EqualError(t, err, "invalid Nuts API URL")
	})
	t.Run("subject not set", func(t *testing.T) {
		c := Config{
			API: APIConfig{
				URL: "http://nutsnode:8081",
			},
			Public:           PublicConfig{URL: "http://nutsnode:8080"},
			DiscoveryService: "discovery",
		}
		err := c.Validate()
		require.EqualError(t, err, "invalid/empty Nuts subject")
	})
	t.Run("ok", func(t *testing.T) {
		c := Config{
			API: APIConfig{
				URL: "http://nutsnode:8081",
			},
			Public:           PublicConfig{URL: "http://nutsnode:8080"},
			DiscoveryService: "discovery",
			OwnSubject:       "sub",
		}
		err := c.Validate()
		require.NoError(t, err)
	})
}
