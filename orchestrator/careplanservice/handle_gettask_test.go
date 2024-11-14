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
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
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

func TestService_handleSearchTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	careplan1, _ := os.ReadFile("./testdata/careplan-1.json")
	careteam2, _ := os.ReadFile("./testdata/careteam-2.json")
	task1, _ := os.ReadFile("./testdata/task-1.json")
	task2, _ := os.ReadFile("./testdata/task-2.json")

	tests := []struct {
		ctx                    context.Context
		name                   string
		queryParams            url.Values
		returnedBundle         *fhir.Bundle
		errorFromRead          error
		returnedCarePlanBundle *fhir.Bundle
		errorFromCarePlanRead  error
		// used to validate result filtering, if needed
		expectedBundle *fhir.Bundle
		expectError    bool
	}{
		{
			ctx:         context.Background(),
			name:        "Empty bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         context.Background(),
			name:        "fhirclient error - task",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:  context.Background(),
			name: "Task returned, no auth",
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1,
					},
				},
			},
			expectError: true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Task returned, auth, task owner",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Task returned, auth, not task owner, error from careplan read",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
			errorFromRead:         nil,
			errorFromCarePlanRead: errors.New("error"),
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectError: false,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Task returned, auth, not task owner, participant in CareTeam",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
			errorFromRead: nil,
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careplan1,
					},
					{
						Resource: careteam2,
					},
				},
			},
			expectError: false,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:        "Task returned, auth, not task owner, participant not in CareTeam",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
			errorFromRead: nil,
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careplan1,
					},
					{
						Resource: careteam2,
					},
				},
			},
			expectError: false,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
				return tt.errorFromRead
			})
			if tt.returnedCarePlanBundle != nil || tt.errorFromCarePlanRead != nil {
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCarePlanBundle))
					return tt.errorFromCarePlanRead
				})
			}

			got, err := service.handleSearchTask(tt.ctx, tt.queryParams, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expectedBundle != nil {
					require.Equal(t, tt.expectedBundle, got)
					return
				}
				require.Equal(t, tt.returnedBundle, got)
			}
		})
	}
}
