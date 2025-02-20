package ehr

import (
	"context"
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
			expectedError: errors.New("failed to create task notification bundle: fetch error"),
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
