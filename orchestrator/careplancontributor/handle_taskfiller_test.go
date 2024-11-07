package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleTaskFillerCreate(t *testing.T) {
	tests := []struct {
		name             string
		ctx              context.Context
		profile          profile.Provider
		notificationTask *fhir.Task
		primaryTask      *fhir.Task
		serviceRequest   *fhir.ServiceRequest
		expectedError    error
		numBundlesPosted int
		mock             func(*mock.MockClient)
	}{
		{
			name:             "primary task, owner = local organization, triggers subtask creation",
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			notificationTask: &primaryTask,
			numBundlesPosted: 1, // One subtask should be created
		},
		{
			name:             "primary task, owner != local organization, nothing should happen",
			profile:          profile.TestProfile{Principal: auth.TestPrincipal2},
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			notificationTask: &primaryTask,
		},
		{
			name: "(error): primary task, invalid (missing SCP profile)",
			notificationTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Meta.Profile = []string{"SomeOtherProfile"}
				return &copiedTask
			}(),
		},
		{
			name: "(error): primary task, without Requester",
			notificationTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Requester = nil
				return &copiedTask
			}(),
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.requester is required but not provided"),
		},
		{
			name: "(error): primary task, without Owner",
			notificationTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Owner = nil
				return &copiedTask
			}(),
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.owner is required but not provided"),
		},
		{
			name: "primary task, status=in-progress, should not process",
			notificationTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusInProgress
				return &copiedTask
			}(),
		},
		{
			name: "(error): primary task, unknown service is requested (primary Task.focus(ServiceRequest).code is not supported)",
			serviceRequest: func() *fhir.ServiceRequest {
				result := deepCopy(serviceRequest)
				result.Code.Coding[0].Code = to.Ptr("UnknownServiceCode")
				return &result
			}(),
			expectedError: errors.New("failed to process new primary Task: ServiceRequest.code does not match any offered services"),
		},
		{
			name: "(error): primary task, without basedOn",
			notificationTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.BasedOn = nil
				return &copiedTask
			}(),
			expectedError: errors.New("task is not valid - skipping: validation errors: Task.basedOn is required but not provided"),
		},
		{
			name:             "subtask status=completed, primary task should be accepted",
			notificationTask: subTask,
			mock: func(client *mock.MockClient) {
				client.EXPECT().
					Update("Task/primary", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, updatedPrimaryTask *fhir.Task, _ interface{}, options ...fhirclient.Option) error {
						assert.Equal(t, fhir.TaskStatusAccepted, updatedPrimaryTask.Status)
						return nil
					})
			},
		},
		{
			name:             "subtask status=completed, primary task status=accepted (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusAccepted
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=in-progress (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusInProgress
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=completed (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusCompleted
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=failed (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusFailed
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=on-hold (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusOnHold
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=cancelled (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusCancelled
				return &copiedTask
			}(),
		},
		{
			name:             "subtask status=completed, primary task status=ready (nothing should be done)",
			notificationTask: subTask,
			primaryTask: func() *fhir.Task {
				copiedTask := deepCopy(primaryTask)
				copiedTask.Status = fhir.TaskStatusReady
				return &copiedTask
			}(),
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockFHIRClient := mock.NewMockClient(ctrl)
			service := &Service{
				workflows:           taskengine.DefaultWorkflows(),
				questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
				cpsClientFactory: func(baseURL *url.URL) fhirclient.Client {
					return mockFHIRClient
				},
			}
			if tt.mock != nil {
				tt.mock(mockFHIRClient)
			}

			// Set up tested FHIR resources
			primaryTask := deepCopy(primaryTask)
			if tt.primaryTask != nil {
				primaryTask = *tt.primaryTask
			}
			serviceRequest := deepCopy(serviceRequest)
			if tt.serviceRequest != nil {
				serviceRequest = *tt.serviceRequest
			}
			notifiedTask := tt.notificationTask
			if notifiedTask == nil {
				notifiedTask = &primaryTask
			}
			// Set up tested context
			service.profile = profile.TestProfile{
				Principal: auth.TestPrincipal1,
			}
			if tt.profile != nil {
				service.profile = tt.profile
			}
			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			mockFHIRClient.EXPECT().
				Read("ServiceRequest/"+*serviceRequest.Id, gomock.Any(), gomock.Any()).
				DoAndReturn(func(id string, result *fhir.ServiceRequest, options ...fhirclient.Option) error {
					*result = serviceRequest
					return nil
				}).AnyTimes()
			mockFHIRClient.EXPECT().
				Read("Task/"+*primaryTask.Id, gomock.Any(), gomock.Any()).
				DoAndReturn(func(id string, result *fhir.Task, options ...fhirclient.Option) error {
					*result = primaryTask
					return nil
				}).AnyTimes()
			if tt.numBundlesPosted > 0 {
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
						_ = json.Unmarshal(bytes, &result)
						return nil
					}).
					Times(tt.numBundlesPosted)
			}

			err := service.handleTaskFillerCreateOrUpdate(ctx, mockFHIRClient, notifiedTask)
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

	// Create the service with the mock FHIR client
	service := &Service{
		workflows:           taskengine.DefaultWorkflows(),
		questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
	}

	taskBytes, _ := json.Marshal(primaryTask)
	var task fhir.Task
	json.Unmarshal(taskBytes, &task)
	workflow := service.workflows["http://snomed.info/sct|719858009"]["http://snomed.info/sct|13645005"].Start()
	questionnaire, err := service.questionnaireLoader.Load(workflow.QuestionnaireUrl)
	require.NoError(t, err)
	require.NotNil(t, questionnaire)

	questionnaireRef := "urn:uuid:" + questionnaire["id"].(string)
	log.Info().Msgf("Creating a new Enrollment Criteria subtask - questionnaireRef: %s", questionnaireRef)
	subtask := service.getSubTask(&primaryTask, questionnaireRef, true)

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
			Reference: to.Ptr("Task/" + *primaryTask.Id),
		},
	}

	require.Equal(t, expectedPartOf, *(&subtask.PartOf), "Task.partOf should be copied from the primary task")

	require.Equal(t, primaryTask.Requester, subtask.Owner, "Task.requester should become Task.owner")
	require.Equal(t, primaryTask.Owner, subtask.Requester, "Task.owner should become Task.requester")
	require.Equal(t, primaryTask.BasedOn, subtask.BasedOn, "Task.basedOn should be copied from the primary task")
	require.Equal(t, primaryTask.Focus, subtask.Focus, "Task.focus should be copied from the primary task")
	require.Equal(t, primaryTask.For, subtask.For, "Task.for should be copied from the primary task")
	require.Equal(t, 1, len(primaryTask.Input), "Subtask should contain one input")
	require.Equal(t, expectedSubTaskInput, subtask.Input, "Subtask should contain a reference to the questionnaire")
}

func deepCopy[T any](src T) T {
	var dst T
	bytes, err := json.Marshal(src)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bytes, &dst)
	if err != nil {
		panic(err)
	}
	return dst
}

var serviceRequest = fhir.ServiceRequest{
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

var primaryTask = fhir.Task{
	Id: to.Ptr("primary"),
	Meta: &fhir.Meta{
		Profile: []string{coolfhir.SCPTaskProfile},
	},
	Status: fhir.TaskStatusRequested,
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

var subTask = func() *fhir.Task {
	subTask := deepCopy(primaryTask)
	swap := subTask.Owner
	subTask.Owner = subTask.Requester
	subTask.Requester = swap
	subTask.PartOf = []fhir.Reference{
		{
			Reference: to.Ptr("Task/" + *primaryTask.Id),
		},
	}
	subTask.Id = to.Ptr("subtask")
	subTask.Status = fhir.TaskStatusCompleted
	return &subTask
}()
