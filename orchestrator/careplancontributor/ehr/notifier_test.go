package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/google/uuid"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestNotifyTaskAccepted(t *testing.T) {
	ctx := context.Background()
	taskId := uuid.NewString()
	patientId := uuid.NewString()
	focusReqId := uuid.NewString()
	questionnaireId := uuid.NewString()
	questionnaireResp1Id := uuid.NewString()
	questionnaireResp2Id := uuid.NewString()
	carePlanId := uuid.NewString()
	patientRef := "Patient/" + patientId
	serviceReqRef := "ServiceRequest/" + focusReqId
	questionnaireRef := "Questionnaire/" + questionnaireId
	carePlanRef := "CarePlan/" + carePlanId
	primaryTask := fhir.Task{
		Id:      &taskId,
		BasedOn: []fhir.Reference{{Reference: &carePlanRef}},
		For:     &fhir.Reference{Reference: &patientRef},
		Focus:   &fhir.Reference{Reference: &serviceReqRef},
		Input: []fhir.TaskInput{{
			ValueReference: &fhir.Reference{Reference: &questionnaireRef},
		}},
		Output: []fhir.TaskOutput{{
			ValueReference: &fhir.Reference{Reference: &questionnaireResp1Id},
		}, {
			ValueReference: &fhir.Reference{Reference: &questionnaireResp2Id},
		}},
	}
	primaryPatient := fhir.Patient{
		Id: &patientId,
	}
	carePlan := fhir.CarePlan{
		Id: &carePlanId,
	}
	serviceReq := fhir.ServiceRequest{
		Id:      &focusReqId,
		Status:  fhir.RequestStatusActive,
		Subject: fhir.Reference{Reference: &serviceReqRef},
	}
	questionnaire := fhir.Questionnaire{
		Id: &questionnaireId,
	}

	questionnaireResponse1 := fhir.QuestionnaireResponse{
		Id: &questionnaireResp1Id,
	}
	questionnaireResponse2 := fhir.QuestionnaireResponse{
		Id: &questionnaireResp2Id,
	}

	tests := []struct {
		name          string
		task          fhir.Task
		setupMocks    func(*test.StubFHIRClient, *MockServiceBusClient)
		expectedError error
	}{
		{
			name: "successful notification",
			task: primaryTask,
			setupMocks: func(client *test.StubFHIRClient, mockServiceBusClient *MockServiceBusClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan)

				mockServiceBusClient.EXPECT().
					SubmitMessage(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
		{
			name:          "error fetching task",
			task:          primaryTask,
			expectedError: errors.New("fetch error"),
			setupMocks: func(client *test.StubFHIRClient, _ *MockServiceBusClient) {
				client.Error = errors.New("fetch error")
			},
		},
		{
			name: "error sending to ServiceBus",
			task: primaryTask,
			setupMocks: func(client *test.StubFHIRClient, mockServiceBusClient *MockServiceBusClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan)

				mockServiceBusClient.EXPECT().
					SubmitMessage(ctx, gomock.Any(), gomock.Any()).
					Return(errors.New("kafka error"))
			},
			expectedError: errors.New("failed to send task to ServiceBus: kafka error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockServiceBusClient := NewMockServiceBusClient(ctrl)
			fhirClient := &test.StubFHIRClient{}

			if tt.setupMocks != nil {
				tt.setupMocks(fhirClient, mockServiceBusClient)
			}
			notifier := NewNotifier(mockServiceBusClient)

			err := notifier.NotifyTaskAccepted(ctx, fhirClient, &tt.task)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsOfType(t *testing.T) {
	type1 := "Questionnaire"
	type2 := "QuestionnaireResponse"
	type3 := "https://example.com/Questionnaire/123"
	type4 := "https://example.com/QuestionnaireResponse/123"
	type6 := "https://example.com/Questionnaire"
	type5 := "Questionnaire/123"
	tests := []struct {
		name           string
		valueReference *fhir.Reference
		typeName       string
		expected       bool
	}{
		{
			name: "type matches directly",
			valueReference: &fhir.Reference{
				Type: &type1,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "type does not match directly",
			valueReference: &fhir.Reference{
				Type: &type2,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "reference matches with https prefix",
			valueReference: &fhir.Reference{
				Reference: &type3,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does not match with https prefix",
			valueReference: &fhir.Reference{
				Reference: &type4,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "reference matches without https prefix",
			valueReference: &fhir.Reference{
				Reference: &type5,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does match without https prefix",
			valueReference: &fhir.Reference{
				Reference: &type5,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does match without value",
			valueReference: &fhir.Reference{
				Reference: &type6,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "nil reference",
			valueReference: &fhir.Reference{
				Reference: nil,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "trigger a compilation error",
			valueReference: &fhir.Reference{
				Reference: &type4,
			},
			typeName: "(",
			expected: false,
		},
		{
			name: "nil type and reference",
			valueReference: &fhir.Reference{
				Type:      nil,
				Reference: nil,
			},
			typeName: "Questionnaire",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOfType(tt.valueReference, tt.typeName)
			require.Equal(t, tt.expected, result)
		})
	}
}

func marshalToRawMessage(resource any) json.RawMessage {
	marshal, err := json.Marshal(resource)
	if err != nil {
		panic(err)
	}
	message := json.RawMessage{}
	err = json.Unmarshal(marshal, &message)
	if err != nil {
		panic(err)
	}
	return message
}
