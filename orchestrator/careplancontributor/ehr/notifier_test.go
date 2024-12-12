package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/google/uuid"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
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
		setupMocks    func(*mock.MockClient, *MockKafkaClient)
		expectedError error
	}{
		{
			name: "successful notification",
			task: primaryTask,
			setupMocks: func(mockFHIRClient *mock.MockClient, mockKafkaClient *MockKafkaClient) {
				mockFHIRClient.EXPECT().
					Read("Task", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
						*bundle = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: marshalToRawMessage(primaryTask),
								},
							},
						}
						return nil
					}).AnyTimes()
				mockFHIRClient.EXPECT().Read("Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(primaryPatient),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("ServiceRequest", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(serviceReq),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("Questionnaire", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(questionnaire),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("QuestionnaireResponse", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(questionnaireResponse1),
							},
							{
								Resource: marshalToRawMessage(questionnaireResponse2),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(carePlan),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockKafkaClient.EXPECT().
					SubmitMessage(ctx, gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
		{
			name: "error fetching task",
			task: primaryTask,
			setupMocks: func(mockFHIRClient *mock.MockClient, mockKafkaClient *MockKafkaClient) {
				mockFHIRClient.EXPECT().
					Read("Task", gomock.Any(), gomock.Any()).
					Return(errors.New("fetch error"))
			},
			expectedError: errors.New("fetch error"),
		},
		{
			name: "error sending to Kafka",
			task: primaryTask,
			setupMocks: func(mockFHIRClient *mock.MockClient, mockKafkaClient *MockKafkaClient) {
				mockFHIRClient.EXPECT().
					Read("Task", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
						*bundle = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: marshalToRawMessage(primaryTask),
								},
							},
						}
						return nil
					}).AnyTimes()
				mockFHIRClient.EXPECT().Read("Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(primaryPatient),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("ServiceRequest", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(serviceReq),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("Questionnaire", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(questionnaire),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("QuestionnaireResponse", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(questionnaireResponse1),
							},
							{
								Resource: marshalToRawMessage(questionnaireResponse2),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ string, bundle *fhir.Bundle, _ ...fhirclient.Option) error {
					*bundle = fhir.Bundle{
						Entry: []fhir.BundleEntry{
							{
								Resource: marshalToRawMessage(carePlan),
							},
						},
					}
					return nil
				}).AnyTimes()
				mockKafkaClient.EXPECT().
					SubmitMessage(ctx, gomock.Any(), gomock.Any()).
					Return(errors.New("kafka error"))
			},
			expectedError: errors.New("kafka error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockFHIRClient := mock.NewMockClient(ctrl)
			mockKafkaClient := NewMockKafkaClient(ctrl)

			if tt.setupMocks != nil {
				tt.setupMocks(mockFHIRClient, mockKafkaClient)
			}
			notifier := NewNotifier(mockKafkaClient)

			err := notifier.NotifyTaskAccepted(ctx, mockFHIRClient, &tt.task)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
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
