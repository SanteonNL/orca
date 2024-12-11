package careplancontributor

import (
	"github.com/stretchr/testify/assert"
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
	t.Run("missing FHIR base URL", func(t *testing.T) {
		err := Config{
			Enabled: true,
			CarePlanService: CarePlanServiceConfig{
				URL: "http://example.com",
			},
		}.Validate()
		require.EqualError(t, err, "careplancontributor.fhir.baseurl is not configured")
	})
}

func TestDefaultConfig(t *testing.T) {
	t.Run("validate healthDataView Endpoint is disabled by default", func(t *testing.T) {
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
