package careplanservice

import (
	"encoding/json"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var validTask = map[string]interface{}{
	"id":           uuid.NewString(),
	"resourceType": "Task",
	"meta": map[string]interface{}{
		"profile": []interface{}{SCP_TASK_PROFILE},
	},
	"requester": map[string]interface{}{
		"identifier": map[string]interface{}{
			"system": "http://fhir.nl/fhir/NamingSystem/ura",
			"value":  "URA-2",
		},
	},
	"owner": map[string]interface{}{
		"identifier": map[string]interface{}{
			"system": "http://fhir.nl/fhir/NamingSystem/ura",
			"value":  "URA-1",
		},
	},
	"basedOn": []interface{}{
		map[string]interface{}{
			"reference": "CarePlan/cps-careplan-01",
		},
	},
	"focus": map[string]interface{}{
		"reference": "ServiceRequest/cps-servicerequest-telemonitoring",
	},
	"for": map[string]interface{}{
		"identifier": map[string]interface{}{
			"system": "http://fhir.nl/fhir/NamingSystem/bsn",
			"value":  "111222333",
		},
	},
	"input": []interface{}{
		map[string]interface{}{
			"type": map[string]interface{}{
				"coding": []interface{}{
					map[string]interface{}{
						"system":  "http://terminology.hl7.org/CodeSystem/task-input-type",
						"code":    "Reference",
						"display": "Reference",
					},
				},
			},
			"valueReference": map[string]interface{}{
				"reference": "urn:uuid:456",
			},
		},
	},
}

func TestService_handleTaskFiller(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	// Define test cases
	tests := []struct {
		name          string
		task          map[string]interface{}
		expectedError bool
		createTimes   int
	}{
		{
			name:          "Valid SCP Task",
			task:          validTask,
			expectedError: false,
			createTimes:   1, // One subtask should be created
		},
		{
			name: "Invalid Task (Missing Profile)",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask["meta"].(map[string]interface{})["profile"] = []interface{}{"SomeOtherProfile"}
				return copiedTask
			}(),
			expectedError: false, // Should skip since it's not an SCP task
			createTimes:   0,     // No subtask creation expected
		},
		{
			name: "Task without Requester",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				delete(copiedTask, "requester")
				return copiedTask
			}(),
			expectedError: true,
			createTimes:   0, // No subtask creation due to missing requester
		},
		{
			name: "Task without Owner",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				delete(copiedTask, "owner")
				return copiedTask
			}(),
			expectedError: true,
			createTimes:   0, // No subtask creation due to missing owner
		},
		{
			name: "Task without partOf",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				delete(copiedTask, "partOf")
				return copiedTask
			}(),
			expectedError: false,
			createTimes:   1, // One subtask should be created since it's treated as a primary task
		},
		{
			name: "Task without basedOn reference",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				delete(copiedTask, "basedOn")
				return copiedTask
			}(),
			expectedError: true, // Invalid partOf reference should cause an error
			createTimes:   0,    // No subtask creation expected
		},
		{
			name: "Task with partOf reference",
			task: func() map[string]interface{} {
				var copiedTask map[string]interface{}
				bytes, _ := json.Marshal(validTask)
				json.Unmarshal(bytes, &copiedTask)
				copiedTask["partOf"] = []interface{}{
					map[string]interface{}{
						"reference": "Task/cps-task-01",
					},
				}
				return copiedTask
			}(),
			expectedError: false,
			createTimes:   0, //not yet implemented, so no error nor a subtask creation
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			log.Info().Msg("Starting test case: " + tt.name)

			// Set expectations for Create calls based on the test case
			mockFHIRClient.EXPECT().
				Create(gomock.Any(), gomock.Any(), gomock.Any()).
				Times(tt.createTimes) // Expect calls based on the test case

			// Call the actual function being tested
			err := service.handleTaskFiller(tt.task)
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestService_createSubTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	// Create a subtask using the primary task
	questionnaire := service.getHardCodedHomeMonitoringQuestionnaire()
	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	subtask := service.getSubTask(validTask, questionnaireRef)

	partOf := []map[string]interface{}{
		{
			"reference": "Task/" + validTask["id"].(string),
		},
	}

	// Define the expected values for the subtask
	expectedSubTask := map[string]interface{}{
		"id":     subtask["id"],
		"status": "ready",
		"meta": map[string]interface{}{
			"profile": []string{
				SCP_TASK_PROFILE,
			},
		},
		"basedOn":   validTask["basedOn"],
		"partOf":    partOf,
		"focus":     validTask["focus"],
		"for":       validTask["for"],
		"owner":     validTask["requester"], // requester becomes owner
		"requester": validTask["owner"],     // owner becomes requester
		"input": []map[string]interface{}{
			{
				"type": map[string]interface{}{
					"coding": []map[string]interface{}{
						{
							"system":  "http://terminology.hl7.org/CodeSystem/task-input-type",
							"code":    "Reference",
							"display": "Reference",
						},
					},
				},
				"valueReference": map[string]interface{}{
					"reference": questionnaireRef, // reference to the questionnaire
				},
			},
		},
	}

	// Verify that the subtask has the correct fields and values
	require.Equal(t, "Task", subtask["resourceType"], "Subtask should have resourceType Task")
	require.Equal(t, expectedSubTask["owner"], subtask["owner"], "Task.requester should become Task.owner")
	require.Equal(t, expectedSubTask["requester"], subtask["requester"], "Task.owner should become Task.requester")
	require.Equal(t, expectedSubTask["partOf"], subtask["partOf"], "Task.partOf should be copied from the primary task")
	require.Equal(t, expectedSubTask["basedOn"], subtask["basedOn"], "Task.basedOn should be copied from the primary task")
	require.Equal(t, expectedSubTask["focus"], subtask["focus"], "Task.focus should be copied from the primary task")
	require.Equal(t, expectedSubTask["for"], subtask["for"], "Task.for should be copied from the primary task")
	require.Equal(t, expectedSubTask["input"], subtask["input"], "Subtask should contain a reference to the questionnaire")
}
