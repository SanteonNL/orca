package cmd

import (
	"log/slog"
	"os"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("public URL not configured", func(t *testing.T) {
		c := Config{
			Nuts: nuts.Config{
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
		require.Equal(t, slog.LevelInfo, c.LogLevel)
	})
	t.Run("default careplancontributor.sessiontimeout", func(t *testing.T) {
		c, _ := LoadConfig()
		require.Equal(t, 15, int(c.CarePlanContributor.SessionTimeout.Minutes()))
	})
	t.Run("log level is parsed", func(t *testing.T) {
		os.Setenv("ORCA_LOGLEVEL", "debug")
		defer os.Unsetenv("ORCA_LOGLEVEL")
		c, err := LoadConfig()
		require.NoError(t, err)
		require.Equal(t, slog.LevelDebug, c.LogLevel)
	})
	t.Run("maps", func(t *testing.T) {
		type mapEntry struct {
			Key   string `koanf:"key"`
			Value string `koanf:"value"`
		}
		type mapContainer struct {
			Map map[string]mapEntry `koanf:"map"`
		}
		t.Run("empty", func(t *testing.T) {
			target := mapContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Empty(t, target.Map)
		})
		t.Run("single", func(t *testing.T) {
			os.Setenv("ORCA_MAP_0_KEY", "foo")
			defer os.Unsetenv("ORCA_MAP_0_KEY")
			os.Setenv("ORCA_MAP_0_VALUE", "bar")
			defer os.Unsetenv("ORCA_MAP_0_VALUE")
			target := mapContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Equal(t, map[string]mapEntry{"0": {Key: "foo", Value: "bar"}}, target.Map)
		})
	})
	t.Run("slices", func(t *testing.T) {
		type sliceContainer struct {
			Slice []string `koanf:"slice"`
		}
		t.Run("empty", func(t *testing.T) {
			os.Setenv("ORCA_SLICE", "")
			target := sliceContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Empty(t, target.Slice)
		})
		t.Run("single", func(t *testing.T) {
			os.Setenv("ORCA_SLICE", "foo")
			target := sliceContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Equal(t, []string{"foo"}, target.Slice)
		})
		t.Run("multiple", func(t *testing.T) {
			os.Setenv("ORCA_SLICE", "foo,bar")
			target := sliceContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Equal(t, []string{"foo", "bar"}, target.Slice)
		})
		t.Run("with escaped comma", func(t *testing.T) {
			os.Setenv("ORCA_SLICE", "foo\\,bar")
			target := sliceContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Equal(t, []string{"foo,bar"}, target.Slice)
		})
		t.Run("spaces are trimmed", func(t *testing.T) {
			os.Setenv("ORCA_SLICE", " foo , bar ")
			target := sliceContainer{}
			require.NoError(t, loadConfigInto(&target))
			require.Equal(t, []string{"foo", "bar"}, target.Slice)
		})
	})
}
