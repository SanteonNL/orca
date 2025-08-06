package ehr

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestNotifier_NotifyTaskAccepted(t *testing.T) {
	ctx := tenants.WithTenant(context.Background(), tenants.Test().Sole())
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
		name  string
		task  fhir.Task
		setup func(*test.StubFHIRClient, *messaging.MemoryBroker)
		// expectedSendMessageError is the error expected when sending a message to the broker.
		expectedSendMessageError error
		// expectedProcessMessageError is the error expected when processing a message from the broker (the message handler creating the bundle).
		expectedProcessMessageError error
	}{
		{
			name: "successful notification",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient, messageBroker *messaging.MemoryBroker) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
			},
		},
		{
			name:                        "error fetching task",
			task:                        primaryTask,
			expectedProcessMessageError: errors.New("event handler *ehr.TaskAcceptedEvent: failed to create task notification bundle: fetch error"),
			setup: func(client *test.StubFHIRClient, messageBroker *messaging.MemoryBroker) {
				client.Error = errors.New("fetch error")
			},
		},
		{
			name: "error sending to message broker",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient, messageBroker *messaging.MemoryBroker) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
				_ = messageBroker.Close(nil)
			},
			expectedSendMessageError: errors.New("event send ehr.TaskAcceptedEvent: no handlers for entity orca.taskengine.task-accepted"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			messageBroker := messaging.NewMemoryBroker()
			fhirClient := &test.StubFHIRClient{}

			bundleTopic := messaging.Entity{Name: "bundle-topic"}
			var capturedBundleJSON string
			require.NoError(t, messageBroker.ReceiveFromQueue(bundleTopic, func(ctx context.Context, message messaging.Message) error {
				capturedBundleJSON = string(message.Body)
				return nil
			}))
			tenantCfg := tenants.Test()

			notifier, _ := NewNotifier(events.NewManager(messageBroker), messageBroker, tenantCfg, bundleTopic, "", func(_ context.Context, _ *url.URL) (fhirclient.Client, *http.Client, error) {
				return fhirClient, nil, nil
			})

			if tt.setup != nil {
				tt.setup(fhirClient, messageBroker)
			}

			err := notifier.NotifyTaskAccepted(ctx, fhirClient.Path().String(), &tt.task)
			if tt.expectedSendMessageError != nil {
				// Bundle instruction message couldn't be sent to receiver
				require.EqualError(t, err, tt.expectedSendMessageError.Error())
			} else {
				// Bundle instruction message is sent to receiver
				require.NoError(t, err)
				if tt.expectedProcessMessageError != nil {
					// Bundle creation should fail
					handlerError := messageBroker.LastHandlerError.Load()
					require.NotNil(t, handlerError)
					require.EqualError(t, *handlerError, tt.expectedProcessMessageError.Error())
				} else {
					// Bundle creation should succeed
					if messageBroker.LastHandlerError.Load() != nil {
						require.NoError(t, *messageBroker.LastHandlerError.Load())
					}
					// Result should be a valid bundle
					var resultBundle BundleSet
					require.NoError(t, json.Unmarshal([]byte(capturedBundleJSON), &resultBundle))
				}
			}
		})
	}
}
