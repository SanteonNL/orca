package cmd

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile/nuts"
	"github.com/stretchr/testify/require"
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
