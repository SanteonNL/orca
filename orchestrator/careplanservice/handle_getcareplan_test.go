package careplanservice

import (
	"context"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"testing"
)

func TestService_handleGetCarePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient:          mockFHIRClient,
		workflows:           taskengine.DefaultWorkflows(),
		questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
	}

	tests := []struct {
		ctx                 context.Context
		name                string
		id                  string
		returnedCarePlan    *fhir.CarePlan
		returnedCareTeam    *fhir.CareTeam
		expectErrorFromRead bool
		expectError         bool
	}{
		{
			ctx:                 context.Background(),
			name:                "CarePlan does not exist",
			id:                  "1",
			returnedCarePlan:    nil,
			returnedCareTeam:    nil,
			expectErrorFromRead: false,
			expectError:         true,
		},
		{
			ctx:  context.Background(),
			name: "No CareTeams returned",
			id:   "1",
			returnedCarePlan: &fhir.CarePlan{
				Id: to.Ptr("1"),
			},
			returnedCareTeam:    nil,
			expectErrorFromRead: false,
			expectError:         true,
		},
		// TODO: Positive test cases, need to figure out how to mock the side effects of mockFHIRClient.EXPECT().Read("CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option)
		// in order to return a CareTeam
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				result = tt.returnedCarePlan
				if tt.expectErrorFromRead {
					return errors.New("error")
				}

				return nil
			})
			got, err := service.handleGetCarePlan(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedCarePlan, got)
			}
		})
	}
}

func TestService_handleSearchCarePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient:          mockFHIRClient,
		workflows:           taskengine.DefaultWorkflows(),
		questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
	}

	tests := []struct {
		ctx                 context.Context
		name                string
		queryParams         url.Values
		returnedBundle      *fhir.Bundle
		expectErrorFromRead bool
		expectError         bool
	}{
		{
			ctx:         context.Background(),
			name:        "No CareTeam in bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectErrorFromRead: false,
			expectError:         true,
		},
		// TODO: Positive test cases, need to figure out how to mock the side effects of mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option)
		// in order to return a CareTeam
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				result = tt.returnedBundle
				if tt.expectErrorFromRead {
					return errors.New("error")
				}

				return nil
			})
			got, err := service.handleSearchCarePlan(tt.ctx, tt.queryParams, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedBundle, got)
			}
		})
	}
}
