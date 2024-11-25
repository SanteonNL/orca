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

func TestService_handleGetCondition(t *testing.T) {
	task1Raw, _ := os.ReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	patient1Raw, _ := os.ReadFile("./testdata/patient-1.json")
	var patient1 fhir.Patient
	_ = json.Unmarshal(patient1Raw, &patient1)

	tests := []TestStruct[fhir.Condition]{
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "error: Condition does not exist",
			id:            "1",
			resourceType:  "Condition",
			errorFromRead: errors.New("fhir error: Condition not found"),
			expectedError: errors.New("fhir error: Condition not found"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Condition exists, no subject",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Condition exists, subject is not a patient",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("SomethingWrong"),
						Value:  to.Ptr("1"),
					},
				},
			},
			errorFromPatientBundleRead: errors.New("fhir error: Issues searching for patient"),
			expectedError:              errors.New("fhir error: Issues searching for patient"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Condition exists, subject is patient, error fetching patient",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
			errorFromPatientBundleRead: errors.New("fhir error: Issues searching for patient"),
			expectedError:              errors.New("fhir error: Issues searching for patient"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Condition exists, no patient returned",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "error: Condition exists, subject is patient, patient returned, incorrect principal",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
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
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1Raw,
					},
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ok: Condition exists, subject is patient, patient returned, correct principal",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
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
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1Raw,
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
		testHelperHandleGetResource[fhir.Condition](t, tt, service.handleGetCondition)
	}
}
