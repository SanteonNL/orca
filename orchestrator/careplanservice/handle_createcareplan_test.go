package careplanservice

import (
	"bytes"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	fhirclient_mock "github.com/SanteonNL/orca/orchestrator/mocks/github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http/httptest"
	"testing"
)

func TestService_handleCreateCarePlan(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		mockClient := fhirclient_mock.NewMockClient(t)
		service := Service{
			fhirClient:      mockClient,
			maxReadBodySize: 1024 * 1024,
		}
		createdCarePlan := fhir.CarePlan{
			CareTeam: []fhir.Reference{
				{
					Reference: to.Ptr("CareTeam/123"),
				},
			},
		}
		txResult := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CarePlan/123"),
						Status:   "204 Created",
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CareTeam/123"),
						Status:   "204 Created",
					},
				},
			},
		}
		mockClient.EXPECT().Read("CarePlan/123", mock.Anything, mock.Anything).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
			data, _ := json.Marshal(createdCarePlan)
			*(result.(*[]byte)) = data
			return nil
		})
		var carePlan fhir.CarePlan
		carePlanBytes, _ := json.Marshal(carePlan)
		httpRequest := httptest.NewRequest("POST", "/CarePlan", bytes.NewReader(carePlanBytes))

		tx := coolfhir.Transaction()
		result, err := service.handleCreateCarePlan(httpRequest, tx)

		require.NoError(t, err)

		// Assert it creates a CareTeam and CarePlan
		require.Len(t, tx.Entry, 2)
		assert.Equal(t, "CarePlan", tx.Entry[0].Request.Url)
		assert.Equal(t, "CareTeam", tx.Entry[1].Request.Url)

		// Process result
		require.NotNil(t, result)
		response, notifications, err := result(&txResult)
		require.NoError(t, err)
		assert.Empty(t, notifications)
		require.Equal(t, "CarePlan/123", *response.Response.Location)
		require.Equal(t, "204 Created", response.Response.Status)
	})
}
