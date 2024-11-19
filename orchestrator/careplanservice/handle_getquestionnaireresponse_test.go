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

func TestService_handleGetQuestionnaireResponse(t *testing.T) {
	task1Raw, _ := os.ReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := []TestStruct[fhir.QuestionnaireResponse]{
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "error: QuestionnaireResponse does not exist",
			id:            "1",
			resourceType:  "QuestionnaireResponse",
			errorFromRead: errors.New("fhir error: QuestionnaireResponse not found"),
			expectedError: errors.New("fhir error: QuestionnaireResponse not found"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: QuestionnaireResponse exists, no basedOn",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "QuestionnaireResponse has invalid number of BasedOn values",
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: QuestionnaireResponse exists, invalid basedOn",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id:      to.Ptr("1"),
				BasedOn: []fhir.Reference{{}},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "QuestionnaireResponse has invalid BasedOn Reference",
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: QuestionnaireResponse exists, not based on Task",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("SomethingElse/1"),
					},
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "QuestionnaireResponse BasedOn is not a Task",
				StatusCode: http.StatusInternalServerError,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: QuestionnaireResponse exists, error fetching task",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId:    "1",
			errorFromTaskRead: errors.New("fhir error: Task not found"),
			expectedError:     errors.New("fhir error: Task not found"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "error: QuestionnaireResponse exists, fetched task, incorrect principal",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId: "1",
			returnedTask:   &task1,
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
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ok: QuestionnaireResponse exists, fetched task, task owner",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId: "1",
			returnedTask:   &task1,
			expectedError:  nil,
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
		testHelperHandleGetResource[fhir.QuestionnaireResponse](t, tt, service.handleGetQuestionnaireResponse)
	}
}
