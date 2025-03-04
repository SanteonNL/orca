package ehr

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/google/uuid"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestNotifyTaskAccepted(t *testing.T) {
	ctx := context.Background()
	taskId := uuid.NewString()
	subtaskId := uuid.NewString()
	patientId := uuid.NewString()
	focusReqId := uuid.NewString()
	questionnaireId := uuid.NewString()
	questionnaireResp1Id := uuid.NewString()
	questionnaireResp2Id := uuid.NewString()
	carePlanId := uuid.NewString()
	careTeamId := uuid.NewString()
	patientRef := "Patient/" + patientId
	serviceReqRef := "ServiceRequest/" + focusReqId
	questionnaireRef := "Questionnaire/" + questionnaireId
	carePlanRef := "CarePlan/" + carePlanId
	careTeamRef := "CareTeam/" + careTeamId
	primaryTaskRef := "Task/" + taskId
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
	secondaryTask := fhir.Task{
		Id:     &subtaskId,
		PartOf: []fhir.Reference{{Reference: &primaryTaskRef}},
	}
	primaryPatient := fhir.Patient{
		Id: &patientId,
	}
	carePlan := fhir.CarePlan{
		Id: &carePlanId,
		Subject: fhir.Reference{
			Reference: &patientRef,
		},
		CareTeam: []fhir.Reference{
			{Reference: &careTeamRef},
		},
	}
	careTeam := fhir.CareTeam{
		Id: &careTeamId,
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
		setupMocks    func(*test.StubFHIRClient, *messaging.MockBroker)
		expectedError error
	}{
		{
			name: "successful notification",
			task: primaryTask,
			setupMocks: func(client *test.StubFHIRClient, mockMessageBroker *messaging.MockBroker) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)

				mockMessageBroker.EXPECT().
					SendMessage(ctx, "test-topic", gomock.Any()).
					Return(nil)
			},
		},
		{
			name:          "error fetching task",
			task:          primaryTask,
			expectedError: errors.New("failed to create task notification bundle: fetch error"),
			setupMocks: func(client *test.StubFHIRClient, _ *messaging.MockBroker) {
				client.Error = errors.New("fetch error")
			},
		},
		{
			name: "error sending to message broker",
			task: primaryTask,
			setupMocks: func(client *test.StubFHIRClient, mockMessageBroker *messaging.MockBroker) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)

				mockMessageBroker.EXPECT().
					SendMessage(ctx, "test-topic", gomock.Any()).
					Return(errors.New("broker error"))
			},
			expectedError: errors.New("failed to send task to message broker: broker error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockMessageBroker := messaging.NewMockBroker(ctrl)
			fhirClient := &test.StubFHIRClient{}

			if tt.setupMocks != nil {
				tt.setupMocks(fhirClient, mockMessageBroker)
			}
			notifier := NewNotifier(mockMessageBroker, "test-topic")

			err := notifier.NotifyTaskAccepted(ctx, fhirClient, &tt.task)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
