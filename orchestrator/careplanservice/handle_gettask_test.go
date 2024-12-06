package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/url"
	"os"
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

	tests := []TestHandleGetStruct[fhir.Task]{
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
	careplan1, _ := os.ReadFile("./testdata/careplan-1.json")
	careteam2, _ := os.ReadFile("./testdata/careteam-2.json")
	task1, _ := os.ReadFile("./testdata/task-1.json")
	task2, _ := os.ReadFile("./testdata/task-2.json")

	tests := []TestHandleSearchStruct[fhir.Task]{
		{
			ctx:           context.Background(),
			resourceType:  "Task",
			name:          "No auth",
			expectedError: errors.New("not authenticated"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Task",
			name:         "Empty bundle",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Task",
			name:         "fhirclient error - task",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromSearch: errors.New("error"),
			expectedError:   errors.New("error"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Task",
			name:         "Task returned, auth, task owner",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1,
					},
				},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Task",
			name:         "Task returned, auth, not task owner, error from careplan read",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
			errorFromCarePlanRead: errors.New("error"),
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Task",
			name:         "Task returned, auth, not task owner, participant in CareTeam",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
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
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			resourceType: "Task",
			name:         "Task returned, auth, not task owner, participant not in CareTeam",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task2,
					},
				},
			},
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
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
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
		t.Run(tt.name, func(t *testing.T) {
			tt.mockClient = mockFHIRClient
			testHelperHandleSearchResource[fhir.Task](t, tt, service.handleSearchTask)
		})
	}
}
