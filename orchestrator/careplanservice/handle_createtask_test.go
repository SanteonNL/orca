package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"reflect"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_basedOn(t *testing.T) {
	type args struct {
		task fhir.Task
	}
	tests := []struct {
		name    string
		args    args
		want    *string
		wantErr error
	}{
		{
			name: "basedOn references a CarePlan (OK)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("CarePlan/123"),
						},
					},
				},
			},
			want:    to.Ptr("CarePlan/123"),
			wantErr: nil,
		},
		{
			name: "no basedOn",
			args: args{
				task: fhir.Task{},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn contains multiple references (instead of 1)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{},
						{},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn does not reference a CarePlan",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("Patient/2"),
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
		{
			name: "basedOn is not a relative reference",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Type:       to.Ptr("CarePlan"),
							Identifier: &fhir.Identifier{},
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := basedOn(tt.args.task)
			if tt.wantErr != nil {
				require.EqualError(t, gotErr, tt.wantErr.Error())
			}
			require.Equal(t, tt.want, got)
		})
	}
}

// TODO: CreatePlan auth
func Test_handleCreateTask_NoExistingCarePlan(t *testing.T) {
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

	tests := []struct {
		ctx            context.Context
		name           string
		taskToCreate   fhir.Task
		createdTask    fhir.Task
		returnedBundle *fhir.Bundle
		errorFromRead  error
		expectError    bool
	}{
		{
			ctx:  context.Background(),
			name: "CreateTask - not authorised",
			taskToCreate: fhir.Task{
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CreateTask - invalid field",
			taskToCreate: fhir.Task{
				Intent:    "invalid",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CreateTask",
			taskToCreate: fhir.Task{
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			createdTask: fhir.Task{
				Id: to.Ptr("123"),
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Response: &fhir.BundleEntryResponse{
							Location: to.Ptr("CarePlan/1"),
							Status:   "204 Created",
						},
					},
					{
						Response: &fhir.BundleEntryResponse{
							Location: to.Ptr("CareTeam/2"),
							Status:   "204 Created",
						},
					},
					{
						Response: &fhir.BundleEntryResponse{
							Location: to.Ptr("Task/3"),
							Status:   "204 Created",
						},
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Task
			taskBytes, _ := json.Marshal(tt.taskToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Task",
				ResourceData: taskBytes,
				HttpMethod:   "POST",
			}

			tx := coolfhir.Transaction()

			result, err := service.handleCreateTask(tt.ctx, fhirRequest, tx)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().Read("Task/3", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				data, _ := json.Marshal(tt.createdTask)
				*(result.(*[]byte)) = data
				return tt.errorFromRead
			})

			require.Len(t, tx.Entry, len(tt.returnedBundle.Entry))

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(tt.returnedBundle)
			require.NoError(t, err)
			assert.Len(t, notifications, 1)
			require.Equal(t, "Task/3", *response.Response.Location)
			require.Equal(t, "204 Created", response.Response.Status)
		})
	}
}

func Test_handleCreateTask_ExistingCarePlan(t *testing.T) {
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

	tests := []struct {
		ctx                   context.Context
		name                  string
		taskToCreate          fhir.Task
		createdTask           fhir.Task
		returnedCarePlan      *fhir.CarePlan
		returnedCarePlanError error
		returnedCareTeams     []fhir.CareTeam
		returnedBundle        *fhir.Bundle
		errorFromRead         error
		expectError           bool
	}{
		{
			ctx:  context.Background(),
			name: "CreateTask - not authorised",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CreateTask - invalid field",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "invalid",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CreateTask - CarePlan not found",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/999"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedCarePlan:      nil,
			returnedCarePlanError: errors.New("not found"),
			returnedBundle:        &fhir.Bundle{},
			errorFromRead:         nil,
			expectError:           true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name: "CreateTask - No CareTeam in CarePlan",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			},
			returnedCarePlan: &fhir.CarePlan{
				Id: to.Ptr("1"),
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("CareTeam/2"),
					},
				},
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		// TODO: Testing this has gotten incredibly complex with the reflection being used and the opts being passed to the Read method.
		// refactor this to full http client tests
		// in the meantime, this functionality is tested in the integ and e2e tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Task
			taskBytes, _ := json.Marshal(tt.taskToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Task",
				ResourceData: taskBytes,
				HttpMethod:   "POST",
			}

			tx := coolfhir.Transaction()

			if tt.returnedCarePlan != nil || tt.returnedCarePlanError != nil {
				mockFHIRClient.EXPECT().Read(*tt.taskToCreate.BasedOn[0].Reference, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ string, resultResource interface{}, opts ...fhirclient.Option) error {
						// The FHIR client reads the resource from the FHIR server, to return it to the client.
						// In this test, we return the expected ServiceRequest.
						if tt.returnedCarePlan != nil {
							reflect.ValueOf(resultResource).Elem().Set(reflect.ValueOf(*tt.returnedCarePlan))
						}
						return tt.returnedCarePlanError
					})
			}

			result, err := service.handleCreateTask(tt.ctx, fhirRequest, tx)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().Read("Task/3", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				data, _ := json.Marshal(tt.createdTask)
				*(result.(*[]byte)) = data
				return tt.errorFromRead
			})

			// Assert it creates the right amount of resources
			require.Len(t, tx.Entry, len(tt.returnedBundle.Entry))

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(tt.returnedBundle)
			require.NoError(t, err)
			assert.Len(t, notifications, 1)
			require.Equal(t, "Task/3", *response.Response.Location)
			require.Equal(t, "204 Created", response.Response.Status)
		})
	}
}
