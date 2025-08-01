package careplancontributor

import (
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		err := Config{}.Validate()
		require.NoError(t, err)
	})
	t.Run("static bearer token in strict mode", func(t *testing.T) {
		globals.StrictMode = true
		defer func() { globals.StrictMode = false }()
		err := Config{
			Enabled:           true,
			StaticBearerToken: "some-token",
		}.Validate()
		require.EqualError(t, err, "staticbearertoken is not allowed in strict mode")
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("validate healthDataView Hostname is disabled by default", func(t *testing.T) {
		config := DefaultConfig()
		assert.False(t, config.HealthDataViewEndpointEnabled)
	})
}

func TestTaskFillerEngineConfig_Validate(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		err := TaskFillerConfig{
			QuestionnaireSyncURLs: []string{"http://example.com", "https://example.com", "file://example.com"},
		}.Validate()
		require.NoError(t, err)
	})
	t.Run("invalid URL", func(t *testing.T) {
		err := TaskFillerConfig{
			QuestionnaireSyncURLs: []string{"ftp://example.com"},
		}.Validate()
		require.EqualError(t, err, "questionnairesyncurls must be http, https or file URLs")
	})
}
