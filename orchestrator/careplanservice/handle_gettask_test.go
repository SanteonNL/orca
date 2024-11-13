package careplanservice

import (
	"context"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"reflect"
	"testing"
)

func TestService_handleGetTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	tests := []struct {
		ctx              context.Context
		name             string
		id               string
		returnedTask     *fhir.Task
		returnedCarePlan *fhir.CarePlan
		returnedCareTeam *fhir.CareTeam
		errorFromRead    error
		expectError      bool
	}{
		{
			ctx:              context.Background(),
			name:             "Task does not exist",
			id:               "1",
			returnedTask:     nil,
			returnedCarePlan: nil,
			returnedCareTeam: nil,
			errorFromRead:    errors.New("error"),
			expectError:      true,
		},
		// TODO: These are complex to mock with the side effects of fhir.QueryParam, refactor unit tests to http tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedCarePlan != nil {
				mockFHIRClient.EXPECT().Read("CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCarePlan))
					return tt.errorFromRead
				})
			}
			mockFHIRClient.EXPECT().Read("Task/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if tt.returnedTask != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedTask))
				}
				return tt.errorFromRead
			})
			got, err := service.handleGetTask(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedTask, got)
			}
		})
	}
}

// TODO: Tests were incorrect, will write new ones as part of the auth fix
