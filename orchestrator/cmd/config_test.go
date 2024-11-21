package cmd

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("public URL not configured", func(t *testing.T) {
		c := Config{
			Nuts: nuts.Config{
				OwnSubject:       "foo",
				DiscoveryService: "test",
				API: nuts.APIConfig{
					URL: "http://example.com",
				},
				Public: nuts.PublicConfig{
					URL: "http://example.com",
				},
			},
			Public:              InterfaceConfig{},
			CarePlanContributor: careplancontributor.Config{},
			CarePlanService:     careplanservice.Config{},
		}
		err := c.Validate()
		require.EqualError(t, err, "public base URL is not configured")
	})
}

func TestLoadConfig(t *testing.T) {
	t.Run("default log level", func(t *testing.T) {
		c, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, zerolog.InfoLevel, c.LogLevel)
	})
	t.Run("default careplancontributor.sessiontimeout", func(t *testing.T) {
		c, _ := LoadConfig()
		require.Equal(t, 15, int(c.CarePlanContributor.SessionTimeout.Minutes()))
	})
	t.Run("log level is parsed", func(t *testing.T) {
		os.Setenv("ORCA_LOGLEVEL", "trace")
		defer os.Unsetenv("ORCA_LOGLEVEL")
		c, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, zerolog.TraceLevel, c.LogLevel)
	})
}
