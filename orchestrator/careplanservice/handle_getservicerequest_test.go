package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"os"
	"testing"
)

func TestService_handleGetServiceRequest(t *testing.T) {
	task1Raw, _ := os.ReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := []TestStruct[fhir.ServiceRequest]{
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "error: ServiceRequest does not exist",
			id:            "1",
			resourceType:  "ServiceRequest",
			errorFromRead: errors.New("fhir error: ServiceRequest not found"),
			expectedError: errors.New("fhir error: ServiceRequest not found"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: ServiceRequest exists, error searching for task",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			errorFromTaskBundleRead: errors.New("fhir error: Issue searching for task"),
			expectedError:           errors.New("fhir error: Issue searching for task"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "error: ServiceRequest exists, fetched task, incorrect principal",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			returnedTaskBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1Raw,
					},
				},
			},
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
				Message:    "Participant does not have access to ServiceRequest",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ok: ServiceRequest exists, fetched task, task owner",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			returnedTaskBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1Raw,
					},
				},
			},
			expectedError: nil,
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
		testHelperHandleGetResource[fhir.ServiceRequest](t, tt, service.handleGetServiceRequest)
	}
}
