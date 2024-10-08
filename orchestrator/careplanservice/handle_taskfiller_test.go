package careplanservice

import (
	"encoding/json"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"

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

	// Define test cases
	tests := []struct {
		name          string
		task          *fhir.Task
		expectedError bool
		createTimes   int
	}{
		{
			name:          "Valid SCP Task",
			task:          &validTask,
			expectedError: false,
			createTimes:   1, // One subtask should be created
		},
		{
			name: "Invalid Task (Missing Profile)",
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Meta.Profile = []string{"SomeOtherProfile"}
				return &copiedTask
			}(),
			expectedError: false, // Should skip since it's not an SCP task
			createTimes:   0,     // No subtask creation expected
		},
		{
			name: "Task without Requester",
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Requester = nil
				return &copiedTask
			}(),
			expectedError: true,
			createTimes:   0, // No subtask creation due to missing requester
		},
		{
			name: "Task without Owner",
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.Owner = nil
				return &copiedTask
			}(),
			expectedError: true,
			createTimes:   0, // No subtask creation due to missing owner
		},
		{
			name: "Task without partOf",
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.PartOf = nil
				return &copiedTask
			}(),
			expectedError: false,
			createTimes:   1, // One subtask should be created since it's treated as a primary task
		},
		{
			name: "Task without basedOn reference",
			task: func() *fhir.Task {
				var copiedTask fhir.Task
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask.BasedOn = nil
				return &copiedTask
			}(),
			expectedError: true, // Invalid partOf reference should cause an error
			createTimes:   0,    // No subtask creation expected
		},
		{
			name: "Task with partOf reference",
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
			expectedError: false,
			createTimes:   0, //not yet implemented, so no error nor a subtask creation
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			log.Info().Msg("Starting test case: " + tt.name)

			mockFHIRClient.EXPECT().
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
					json.Unmarshal(bytes, &result)
					return nil
				}).
				Times(tt.createTimes)

			err := service.handleTaskFillerCreate(tt.task)
			if tt.expectedError {
				require.Error(t, err)
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
		fhirClient:          mockFHIRClient,
		workflows:           taskengine.DefaultWorkflows(),
		questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
	}

	taskBytes, _ := json.Marshal(validTask)
	var task fhir.Task
	json.Unmarshal(taskBytes, &task)
	workflow := service.workflows["2.16.528.1.1007.3.3.21514.ehr.orders|99534756439"].Start()
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

var validTask = fhir.Task{
	Id: to.Ptr(uuid.NewString()),
	Meta: &fhir.Meta{
		Profile: []string{coolfhir.SCPTaskProfile},
	},
	Focus: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
			Value:  to.Ptr("99534756439"),
		},
	},
	Requester: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("URA-2"),
		},
	},
	Owner: &fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("URA-1"),
		},
	},
	BasedOn: []fhir.Reference{
		{
			Reference: to.Ptr("CarePlan/cps-careplan-01"),
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
