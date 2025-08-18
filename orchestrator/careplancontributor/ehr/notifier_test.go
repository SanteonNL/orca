package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/google/uuid"
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
		Status:  fhir.TaskStatusAccepted, // Initialize with accepted status
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
		name                     string
		task                     fhir.Task
		setup                    func(*test.StubFHIRClient)
		mockServerSetup          func() *httptest.Server
		expectedError            error
		expectedTaskStatusUpdate bool
		expectedTaskStatus       fhir.TaskStatus
	}{
		{
			name: "successful notification with HTTP 200 response",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
			},
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Verify the request method and content type
					require.Equal(t, "POST", r.Method)
					require.Equal(t, "application/fhir+json", r.Header.Get("Content-Type"))

					// Verify the request body contains BundleSet
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)

					var bundleSet BundleSet
					err = json.Unmarshal(body, &bundleSet)
					require.NoError(t, err)

					w.WriteHeader(http.StatusOK)
				}))
			},
		},
		{
			name: "HTTP 400 bad request with OperationOutcome",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
			},
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					operationOutcome := fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{
							{
								Diagnostics: to.Ptr("Invalid patient data"),
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(operationOutcome)
				}))
			},
			expectedTaskStatusUpdate: true,
			expectedTaskStatus:       fhir.TaskStatusRejected,
		},
		{
			name: "HTTP 500 server error",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
			},
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectedError: errors.New("failed to send task to endpoint, status code: 500"),
		},
		{
			name: "error creating task notification bundle",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient) {
				client.Error = errors.New("fetch error")
			},
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			expectedError: errors.New("failed to create task notification bundle: fetch error"),
		},
		{
			name: "HTTP endpoint unreachable",
			task: primaryTask,
			setup: func(client *test.StubFHIRClient) {
				client.Resources = append(client.Resources, primaryTask, primaryPatient, serviceReq,
					questionnaire, questionnaireResponse1, questionnaireResponse2, carePlan, secondaryTask, careTeam)
			},
			mockServerSetup: func() *httptest.Server {
				// Return a server that we'll immediately close to simulate unreachable endpoint
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close()
				return server
			},
			expectedError: errors.New("failed to send task to endpoint"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Set up mock HTTP server for the endpoint
			mockServer := tt.mockServerSetup()
			defer mockServer.Close()

			messageBroker := messaging.NewMemoryBroker()
			fhirClient := &test.StubFHIRClient{}
			tenantCfg := tenants.Test()

			if tt.setup != nil {
				tt.setup(fhirClient)
			}

			notifier, err := NewNotifier(events.NewManager(messageBroker), tenantCfg, mockServer.URL, func(_ context.Context, _ *url.URL) (fhirclient.Client, *http.Client, error) {
				return fhirClient, nil, nil
			})
			require.NoError(t, err)

			// Execute the notification
			err = notifier.NotifyTaskAccepted(ctx, fhirClient.Path().String(), &tt.task)

			// Check expectations
			if tt.expectedError != nil {
				require.Error(t, err)
				require.True(t, strings.Contains(err.Error(), strings.Split(tt.expectedError.Error(), ":")[0]))
			} else {
				require.NoError(t, err)
			}

			// Check if task status was updated as expected
			if tt.expectedTaskStatusUpdate {
				// Find the updated task in the FHIR client
				var updatedTask *fhir.Task
				for _, resource := range fhirClient.Resources {
					if task, ok := resource.(fhir.Task); ok && *task.Id == taskId {
						updatedTask = &task
						break
					}
				}
				require.NotNil(t, updatedTask, "Expected task to be updated but it wasn't found")
				require.Equal(t, tt.expectedTaskStatus, updatedTask.Status)
			}
		})
	}
}

func TestSendBundle(t *testing.T) {
	tests := []struct {
		name            string
		mockServerSetup func() *httptest.Server
		bundleSet       BundleSet
		expectedError   error
	}{
		{
			name: "successful send",
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
			},
			bundleSet: BundleSet{task: "Task/123"},
		},
		{
			name: "bad request with operation outcome",
			mockServerSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					operationOutcome := fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{
							{
								Diagnostics: to.Ptr("Test error"),
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					json.NewEncoder(w).Encode(operationOutcome)
				}))
			},
			bundleSet:     BundleSet{task: "Task/123"},
			expectedError: &BadRequest{Reason: to.Ptr("Test error")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := tt.mockServerSetup()
			defer mockServer.Close()

			ctx := context.Background()
			err := sendBundle(ctx, mockServer.URL, tt.bundleSet)

			if tt.expectedError != nil {
				require.Error(t, err)
				var badRequest *BadRequest
				if errors.As(tt.expectedError, &badRequest) {
					var actualBadRequest *BadRequest
					require.ErrorAs(t, err, &actualBadRequest)
					require.Equal(t, *badRequest.Reason, *actualBadRequest.Reason)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
