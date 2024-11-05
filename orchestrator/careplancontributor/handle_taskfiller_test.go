package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleTaskFillerCreate(t *testing.T) {
	// Define test cases
	tests := []struct {
		name           string
		ctx            context.Context
		profile        profile.Provider
		task           *fhir.Task
		serviceRequest *fhir.ServiceRequest
		expectedError  error
		createTimes    int
	}{
		{
			name: "Valid SCP Task",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			task:        &validTask,
			createTimes: 1, // One subtask should be created
		},
		{
			name: "Valid SCP Task - Not owner",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal2,
			},
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			task:        &validTask,
			createTimes: 0,
		},
		{
			name: "Invalid Task (Missing Profile)",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Meta.Profile = []string{"SomeOtherProfile"}
				return &copiedTask
			}(),
			expectedError: nil, // Should skip since it's not an SCP task
			createTimes:   0,   // No subtask creation expected
		},
		{
			name: "Task without Requester",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Requester = nil
				return &copiedTask
			}(),
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.requester is required but not provided"),
			createTimes:   0, // No subtask creation due to missing requester
		},
		{
			name: "Task without Owner",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Owner = nil
				return &copiedTask
			}(),
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.owner is required but not provided"),
			createTimes:   0, // No subtask creation due to missing owner
		},
		{
			name: "Task without partOf: created Task is primary Task, triggers create of Subtask with Questionnaire",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.PartOf = nil
				return &copiedTask
			}(),
			createTimes: 1, // One subtask should be created since it's treated as a primary task
		},
		{
			name: "Unknown service is requested (primary Task.focus(ServiceRequest).code is not supported)",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			serviceRequest: func() *fhir.ServiceRequest {
				var result fhir.ServiceRequest
				bytes, _ := json.Marshal(validServiceRequest)
				_ = json.Unmarshal(bytes, &result)
				result.Code.Coding[0].Code = to.Ptr("UnknownServiceCode")
				return &result
			}(),
			expectedError: errors.New("failed to process new primary Task: ServiceRequest.code does not match any offered services"),
			createTimes:   0,
		},
		{
			name: "Task without basedOn reference",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.BasedOn = nil
				return &copiedTask
			}(),
			// Invalid partOf reference should cause an error
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.basedOn is required but not provided"),
			createTimes:   0, // No subtask creation expected
		},
		{
			name: "Last subtask of a workflow is completed, primary Task is completed",
			profile: profile.TestProfile{
				Principal: auth.TestPrincipal1,
			},
			ctx: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.PartOf = []fhir.Reference{
					{
						Reference: to.Ptr("Task/cps-task-01"),
					},
				}
				return &copiedTask
			}(),
			createTimes: 0, //not yet implemented, so no error nor a subtask creation
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			fhirClient := mock.NewMockClient(ctrl)
			service := &Service{
				carePlanServiceClient: fhirClient,
				workflows:             taskengine.DefaultWorkflows(),
				questionnaireLoader:   taskengine.EmbeddedQuestionnaireLoader{},
			}

			service.profile = tt.profile

			serviceRequest := validServiceRequest
			if tt.serviceRequest != nil {
				serviceRequest = *tt.serviceRequest
			}
			fhirClient.EXPECT().
				Read("ServiceRequest/1", gomock.Any(), gomock.Any()).
				DoAndReturn(func(id string, result interface{}, options ...fhirclient.Option) error {
					bytes, _ := json.Marshal(serviceRequest)
					_ = json.Unmarshal(bytes, &result)
					return nil
				}).AnyTimes()
			fhirClient.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(bundle fhir.Bundle, result interface{}, options ...fhirclient.Option) error {
					// Simulate the creation by setting the result to a mock response
					mockResponse := map[string]interface{}{
						"id":           uuid.NewString(),
						"resourceType": "Bundle",
						"type":         "transaction-response",
						"entry": []interface{}{
							map[string]interface{}{
								"response": map[string]interface{}{
									"status":   "201 Created",
									"location": "Task/" + uuid.NewString(),
								},
							},
						},
					}
					bytes, _ := json.Marshal(mockResponse)
					_ = json.Unmarshal(bytes, &result)
					return nil
				}).
				Times(tt.createTimes)

			task := &validTask
			if tt.task != nil {
				task = tt.task
			}

			err := service.handleTaskNotification(tt.ctx, task)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_createSubTaskEnrollmentCriteria(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		carePlanServiceClient: mockFHIRClient,
		workflows:             taskengine.DefaultWorkflows(),
		questionnaireLoader:   taskengine.EmbeddedQuestionnaireLoader{},
	}

	taskBytes, _ := json.Marshal(validTask)
	var task fhir.Task
	json.Unmarshal(taskBytes, &task)
	workflow := service.workflows["http://snomed.info/sct|719858009"]["http://snomed.info/sct|13645005"].Start()
	questionnaire, err := service.questionnaireLoader.Load(workflow.QuestionnaireUrl)
	require.NoError(t, err)
	require.NotNil(t, questionnaire)

	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	log.Info().Msgf("Creating a new Enrollment Criteria subtask - questionnaireRef: %s", questionnaireRef)
	subtask := service.getSubTask(&validTask, questionnaireRef, true)

	expectedSubTaskInput := []fhir.TaskInput{
		{
			Type: fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://terminology.hl7.org/CodeSystem/task-input-type"),
						Code:    to.Ptr("Reference"),
						Display: to.Ptr("Reference"),
					},
				},
			},
			ValueReference: &fhir.Reference{
				Reference: to.Ptr(questionnaireRef), // reference to the questionnaire
			},
		},
	}

	expectedPartOf := []fhir.Reference{
		{
			Reference: to.Ptr("Task/" + *validTask.Id),
		},
	}

	require.Equal(t, expectedPartOf, *(&subtask.PartOf), "Task.partOf should be copied from the primary task")

	require.Equal(t, validTask.Requester, subtask.Owner, "Task.requester should become Task.owner")
	require.Equal(t, validTask.Owner, subtask.Requester, "Task.owner should become Task.requester")
	require.Equal(t, validTask.BasedOn, subtask.BasedOn, "Task.basedOn should be copied from the primary task")
	require.Equal(t, validTask.Focus, subtask.Focus, "Task.focus should be copied from the primary task")
	require.Equal(t, validTask.For, subtask.For, "Task.for should be copied from the primary task")
	require.Equal(t, 1, len(validTask.Input), "Subtask should contain one input")
	require.Equal(t, expectedSubTaskInput, subtask.Input, "Subtask should contain a reference to the questionnaire")
}

var validServiceRequest = fhir.ServiceRequest{
	Id: to.Ptr("1"),
	Code: &fhir.CodeableConcept{
		Coding: []fhir.Coding{
			{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("719858009"),
			},
		},
	},
}

var validTask = fhir.Task{
	Id: to.Ptr(uuid.NewString()),
	Meta: &fhir.Meta{
		Profile: []string{coolfhir.SCPTaskProfile},
	},
	Focus: &fhir.Reference{
		Reference: to.Ptr("ServiceRequest/1"),
	},
	Requester: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("2"),
		},
	},
	Owner: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("1"),
		},
	},
	Intent: "order",
	BasedOn: []fhir.Reference{
		{
			Reference: to.Ptr("CarePlan/cps-careplan-01"),
		},
	},
	ReasonCode: &fhir.CodeableConcept{
		Coding: []fhir.Coding{
			{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("13645005"),
			},
		},
	},
	For: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
			Value:  to.Ptr("111222333"),
		},
	},
	Input: []fhir.TaskInput{
		{
			Type: fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://terminology.hl7.org/CodeSystem/task-input-type"),
						Code:    to.Ptr("Reference"),
						Display: to.Ptr("Reference"),
					},
				},
			},
			ValueReference: &fhir.Reference{
				Reference: to.Ptr("urn:uuid:456"),
			},
		},
	},
}
