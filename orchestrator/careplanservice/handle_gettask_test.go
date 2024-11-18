package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestService_handleGetTask(t *testing.T) {
	task1Raw, _ := os.ReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := []TestStruct[fhir.Task]{
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "error: Task does not exist",
			id:               "1",
			resourceType:     "Task",
			returnedResource: nil,
			errorFromRead:    errors.New("fhir error: task not found"),
			expectedError:    errors.New("fhir error: task not found"),
		},
		{
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:                   "error: Task exists, auth, not owner or requester, error fetching CarePlan",
			id:                     "1",
			resourceType:           "Task",
			returnedResource:       &task1,
			returnedCarePlanBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			errorFromCarePlanRead:  errors.New("fhir error: careplan read failed"),
			expectedError:          errors.New("fhir error: careplan read failed"),
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "error: Task exists, auth, CarePlan and CareTeam returned, not a participant",
			id:               "1",
			resourceType:     "Task",
			returnedResource: &task1,
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1Raw,
					},
					{
						Resource: careTeam2Raw,
					},
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant is not part of CareTeam",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:             "ok: Task exists, auth, CarePlan and CareTeam returned, owner",
			id:               "1",
			resourceType:     "Task",
			returnedResource: &task1,
			expectedError:    nil,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	for _, tt := range tests {
		tt.mockClient = mockFHIRClient
		testHelperHandleGetResource[fhir.Task](t, tt, service.handleGetTask)
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
		expectedBundle         *fhir.Bundle
		expectError            bool
	}{
		{
			ctx:         context.Background(),
			name:        "No auth",
			expectError: true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Empty bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "fhirclient error - task",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: errors.New("error"),
			expectError:   true,
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
			if tt.returnedBundle != nil || tt.errorFromRead != nil {
				mockFHIRClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
					return tt.errorFromRead
				})
			}
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
