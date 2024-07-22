package careplanservice

import (
	"bytes"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	fhirclient_mock "github.com/SanteonNL/orca/orchestrator/mocks/github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http"
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

		var capturedBundle fhir.Bundle
		mockClient.EXPECT().Create(mock.AnythingOfType("fhir.Bundle"), mock.AnythingOfType("*fhir.Bundle"),
			mock.AnythingOfType("PreRequestOption"),
		).RunAndReturn(func(resource interface{}, result interface{}, option ...fhirclient.Option) error {
			var bundle fhir.Bundle
			bundle.Entry = []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CarePlan/123"),
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CareTeam/123"),
					},
				},
			}
			*(result.(*fhir.Bundle)) = bundle
			capturedBundle = resource.(fhir.Bundle)
			return nil
		})
		mockClient.EXPECT().Read("CarePlan/123", mock.AnythingOfType("*fhir.CarePlan"),
			mock.AnythingOfType("PostRequestOption"),
		).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
			*(result.(*fhir.CarePlan)) = createdCarePlan
			return nil
		})
		var carePlan fhir.CarePlan
		carePlanBytes, _ := json.Marshal(carePlan)
		httpResponse := httptest.NewRecorder()
		httpRequest := httptest.NewRequest("POST", "/CarePlan", bytes.NewReader(carePlanBytes))

		err := service.handleCreateCarePlan(httpResponse, httpRequest)

		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, httpResponse.Code)
		// Check that the input Bundle is a transaction with a CarePlan and a CareTeam.
		// The first should be the CarePlan
		require.Len(t, capturedBundle.Entry, 2)
	})

}
