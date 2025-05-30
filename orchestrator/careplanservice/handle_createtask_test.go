package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"net/url"
	"reflect"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

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
	fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
	service := &Service{
		profile:      profile.Test(),
		fhirClient:   mockFHIRClient,
		fhirURL:      fhirBaseUrl,
		eventManager: events.NewManager(messaging.NewMemoryBroker()),
	}

	scpMeta := &fhir.Meta{
		Profile: []string{coolfhir.SCPTaskProfile},
	}
	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("CarePlan/1"),
					Status:   "201 Created",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/4"),
					Status:   "201 Created",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Task/3"),
					Status:   "201 Created",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/5"),
					Status:   "201 Created",
				},
			},
		},
	}
	defaultTask := fhir.Task{
		Intent:    "order",
		Status:    fhir.TaskStatusRequested,
		Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
		Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Meta:      scpMeta,
		For: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	defaultPatient, _ := json.Marshal(&fhir.Patient{
		Id: to.Ptr("1"),
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	})
	tests := []struct {
		name                       string
		taskToCreate               fhir.Task
		createdTask                fhir.Task
		returnedBundle             *fhir.Bundle
		returnedPatientBundle      *fhir.Bundle
		errorFromPatientBundleRead error
		errorFromRead              error
		expectError                error
		principal                  *auth.Principal
	}{
		{
			name: "happy flow",
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultPatient,
					},
				},
			},
		},
		{
			name:      "error: requester is not a local organization",
			principal: auth.TestPrincipal2,
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Requester = coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2")
			}),
			expectError: errors.New("requester must be local care organization in order to create new CarePlan and CareTeam"),
		},
		{
			name: "error: principal is not Task.requester",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Requester = coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "3")
			}),
			expectError: errors.New("requester must be equal to Task.requester"),
		},
		{
			name: "error: invalid 'intent' field",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Intent = "invalid"
			}),
			expectError: errors.New("task.Intent must be 'order'"),
		},
		{
			name: "error: unsecure literal reference",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Focus = &fhir.Reference{
					Reference: to.Ptr("http://example.com/fhir/Patient/1"),
				}
			}),
			expectError: errors.New("literal reference is URL with scheme http://, only https:// is allowed (path=focus.reference)"),
		},
		{
			name: "error: not an SCP Task",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Meta = nil
			}),
			expectError: errors.New("Task is not SCP task"),
		},
		{
			name: "error: status is 'accepted' (new Tasks must be received as 'requested' or `ready`)",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.Status = fhir.TaskStatusAccepted
			}),
			expectError: errors.New("cannot create Task with status accepted, must be requested or ready"),
		},
		{
			name: "error: no Task.for",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.For = nil
			}),
			expectError: errors.New("Task.For must be set with a local reference, or a logical identifier, referencing a patient"),
		},
		{
			name: "Task.for contains a local reference",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.For = &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
				}
			}),
		},
		{
			name: "Task.for contains a logical identifier with BSN",
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultPatient,
					},
				},
			},
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.For = &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				}
			}),
		},
		{
			name:                       "error: Task.for contains a logical identifier with BSN, search for patient fails",
			errorFromPatientBundleRead: errors.New("fhir error: Issues searching for patient"),
			expectError:                errors.New("fhir error: Issues searching for patient"),
		},
		{
			name: "Task.for contains a local reference and a logical identifier with BSN",
			taskToCreate: deep.AlterCopy(defaultTask, func(task *fhir.Task) {
				task.For = &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				}
			}),
		},
		{
			name: "Task location in transaction response bundle contains an absolute URL (Microsoft Azure FHIR behavior)",
			returnedBundle: deep.AlterCopy(defaultReturnedBundle, func(bundle **fhir.Bundle) {
				b := *bundle
				b.Entry[1].Response.Location = to.Ptr(fhirBaseUrl.JoinPath("Task/3").String())
			}),
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultPatient,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if tt.returnedPatientBundle != nil || tt.errorFromPatientBundleRead != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "Patient", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, path string, params url.Values, result interface{}, option ...fhirclient.Option) error {
					if tt.returnedPatientBundle != nil {
						reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedPatientBundle))
					}
					return tt.errorFromPatientBundleRead
				})
			}

			// Create a Task
			var taskToCreate = deep.Copy(defaultTask)
			if !deep.Equal(tt.taskToCreate, fhir.Task{}) {
				taskToCreate = tt.taskToCreate
			}

			taskBytes, _ := json.Marshal(taskToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Task",
				ResourceData: taskBytes,
				HttpMethod:   "POST",
				HttpHeaders: map[string][]string{
					"If-None-Exist": {"ifnoneexist"},
				},
				Principal: auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			tx := coolfhir.Transaction()

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)
			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			result, err := service.handleCreateTask(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = []byte{}
				return tt.errorFromRead
			})

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Task/3", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				data, _ := json.Marshal(tt.createdTask)
				*(result.(*[]byte)) = data
				return tt.errorFromRead
			})

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify CarePlan", func(t *testing.T) {
					carePlanEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("CarePlan"))

					var carePlan fhir.CarePlan
					err := json.Unmarshal(carePlanEntry.Resource, &carePlan)
					require.NoError(t, err)

					require.Equal(t, fhir.RequestStatusActive, carePlan.Status)
					require.Contains(t, *carePlan.Subject.Reference, "Patient/")
					require.Equal(t, taskToCreate.Requester, carePlan.Author)
				})
			}

			t.Run("original FHIR request headers are passed to outgoing Bundle entry", func(t *testing.T) {
				taskEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Task"))
				require.Equal(t, "ifnoneexist", *taskEntry.Request.IfNoneExist)
			})

			// Process result
			require.NotNil(t, result)
			responses, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			assert.Len(t, notifications, 2)
			require.Equal(t, "Task/3", *responses[0].Response.Location)
			require.Equal(t, "201 Created", responses[0].Response.Status)
			require.Len(t, notifications, 2)
			require.IsType(t, &fhir.Task{}, notifications[0])
			require.IsType(t, &fhir.CarePlan{}, notifications[1])
		})
	}
}

func Test_handleCreateTask_ExistingCarePlan(t *testing.T) {

	tests := []struct {
		name                  string
		taskToCreate          fhir.Task
		createdTask           fhir.Task
		returnedCarePlan      *fhir.CarePlan
		returnedCarePlanError error
		returnedCareTeams     []fhir.CareTeam
		returnedBundle        *fhir.Bundle
		errorFromRead         error
		expectError           bool
		principal             *auth.Principal
	}{
		{
			name: "invalid field",
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
				For: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			name: "Not SCP Task",
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
				For: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
		},
		{
			name: "CarePlan not found",
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
				Meta: &fhir.Meta{
					Profile: []string{coolfhir.SCPTaskProfile},
				},
				For: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedCarePlan:      nil,
			returnedCarePlanError: errors.New("not found"),
			returnedBundle:        &fhir.Bundle{},
			errorFromRead:         nil,
			expectError:           true,
		},
		{
			name: "No CareTeam in CarePlan",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "3"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
				Meta: &fhir.Meta{
					Profile: []string{coolfhir.SCPTaskProfile},
				},
				For: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedCarePlan: &fhir.CarePlan{
				Id: to.Ptr("1"),
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("CareTeam/2"),
					},
				},
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
			principal:      auth.TestPrincipal3,
		},
		{
			name: "error: Task.for does not match CarePlan.subject",
			taskToCreate: fhir.Task{
				BasedOn: []fhir.Reference{
					{
						Type:      to.Ptr("CarePlan"),
						Reference: to.Ptr("CarePlan/1"),
					},
				},
				Intent:    "order",
				Status:    fhir.TaskStatusRequested,
				Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "3"),
				Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
				Meta: &fhir.Meta{
					Profile: []string{coolfhir.SCPTaskProfile},
				},
				For: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			returnedCarePlan: &fhir.CarePlan{
				Id: to.Ptr("1"),
				CareTeam: []fhir.Reference{
					{
						Reference: to.Ptr("CareTeam/2"),
					},
				},
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1234567890"),
					},
				},
			},
			returnedBundle: &fhir.Bundle{},
			errorFromRead:  nil,
			expectError:    true,
			principal:      auth.TestPrincipal3,
		},
		// TODO: Testing this has gotten incredibly complex with the reflection being used and the opts being passed to the Read method.
		// refactor this to full http client tests
		// in the meantime, this functionality is tested in the integ and e2e tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create a mock FHIR client using the generated mock
			mockFHIRClient := mock.NewMockClient(ctrl)

			// Create the service with the mock FHIR client
			fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
			service := &Service{
				fhirClient: mockFHIRClient,
				profile:    profile.Test(),
				fhirURL:    fhirBaseUrl,
			}

			// Create a Task
			taskBytes, _ := json.Marshal(tt.taskToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Task",
				ResourceData: taskBytes,
				HttpMethod:   "POST",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			tx := coolfhir.Transaction()

			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

			if tt.returnedCarePlan != nil || tt.returnedCarePlanError != nil {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), *tt.taskToCreate.BasedOn[0].Reference, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, resultResource interface{}, opts ...fhirclient.Option) error {
						// The FHIR client reads the resource from the FHIR server, to return it to the client.
						// In this test, we return the expected ServiceRequest.
						if tt.returnedCarePlan != nil {
							reflect.ValueOf(resultResource).Elem().Set(reflect.ValueOf(*tt.returnedCarePlan))
						}
						return tt.returnedCarePlanError
					})
			}

			result, err := service.handleCreateTask(ctx, fhirRequest, tx)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = []byte{}
				return tt.errorFromRead
			})

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Task/3", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				data, _ := json.Marshal(tt.createdTask)
				*(result.(*[]byte)) = data
				return tt.errorFromRead
			})

			// Assert it creates the right amount of resources
			require.Len(t, tx.Entry, len(tt.returnedBundle.Entry))

			// Process result
			require.NotNil(t, result)
			responses, notifications, err := result(tt.returnedBundle)
			require.NoError(t, err)
			assert.Len(t, notifications, 1)
			require.Equal(t, "Task/3", *responses[0].Response.Location)
			require.Equal(t, "201 Created", responses[0].Response.Status)
		})
	}
}

// TODO: Unit test for creating an SCP subtask, to be done when refactoring the tests to use HTTP client mocking (too complex to test positive cases with reflection)
