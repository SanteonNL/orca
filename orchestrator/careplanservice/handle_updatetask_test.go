package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_isValidTransition(t *testing.T) {
	type args struct {
		from         fhir.TaskStatus
		to           fhir.TaskStatus
		isOwner      bool
		isRequester  bool
		isScpSubtask bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// Positive cases
		{
			name: "requested -> received : owner (OK)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusReceived,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "requested -> accepted : owner (OK)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusAccepted,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "requested -> rejected : owner (OK)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusRejected,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : owner (OK)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusCancelled,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "received -> accepted : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReceived,
				to:           fhir.TaskStatusAccepted,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "received -> rejected : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReceived,
				to:           fhir.TaskStatusRejected,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "received -> cancelled : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReceived,
				to:           fhir.TaskStatusCancelled,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "received -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "received -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "accepted -> in-progress : owner (OK)",
			args: args{
				from:         fhir.TaskStatusAccepted,
				to:           fhir.TaskStatusInProgress,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : owner (OK)",
			args: args{
				from:         fhir.TaskStatusAccepted,
				to:           fhir.TaskStatusCancelled,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "in-progress -> completed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusInProgress,
				to:           fhir.TaskStatusCompleted,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "in-progress -> failed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusInProgress,
				to:           fhir.TaskStatusFailed,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : owner (OK)",
			args: args{
				from:         fhir.TaskStatusInProgress,
				to:           fhir.TaskStatusOnHold,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : requester (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusOnHold,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusOnHold,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : owner (OK)",
			args: args{
				from:         fhir.TaskStatusOnHold,
				to:           fhir.TaskStatusInProgress,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : requester (OK)",
			args: args{
				from:        fhir.TaskStatusOnHold,
				to:          fhir.TaskStatusInProgress,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusOnHold,
				to:          fhir.TaskStatusInProgress,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "ready -> completed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusCompleted,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		{
			name: "ready -> failed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusFailed,
				isOwner:      true,
				isRequester:  false,
				isScpSubtask: false,
			},
			want: true,
		},
		// Negative cases -> Invalid requester/owner
		{
			name: "requested -> received : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusReceived,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "requested -> accepted : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusAccepted,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "requested -> rejected : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusRequested,
				to:           fhir.TaskStatusRejected,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "received -> accepted : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusReceived,
				to:           fhir.TaskStatusAccepted,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "received -> rejected : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusReceived,
				to:           fhir.TaskStatusRejected,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "accepted -> in-progress : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusAccepted,
				to:           fhir.TaskStatusInProgress,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "in-progress -> completed : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusInProgress,
				to:           fhir.TaskStatusCompleted,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "in-progress -> failed : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusInProgress,
				to:           fhir.TaskStatusFailed,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "ready -> completed : requester (FAIL)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusCompleted,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "ready -> failed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusFailed,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: false,
			},
			want: false,
		},
		{
			name: "scp subTask ready -> failed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusFailed,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: true,
			},
			want: false,
		},
		{
			name: "scp subTask ready -> completed : owner (OK)",
			args: args{
				from:         fhir.TaskStatusReady,
				to:           fhir.TaskStatusCompleted,
				isOwner:      false,
				isRequester:  true,
				isScpSubtask: true,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTransition(tt.args.from, tt.args.to, tt.args.isOwner, tt.args.isRequester, tt.args.isScpSubtask)
			require.Equal(t, tt.want, got)
			// Validate that the opposite direction fails, besides in-progress <-> on-hold
			if (tt.args.from == fhir.TaskStatusOnHold && tt.args.to == fhir.TaskStatusInProgress) || (tt.args.from == fhir.TaskStatusInProgress && tt.args.to == fhir.TaskStatusOnHold) {
				return
			}
			got = isValidTransition(tt.args.to, tt.args.from, tt.args.isOwner, tt.args.isRequester, tt.args.isScpSubtask)
			require.Equal(t, false, got)
		})
	}
}

func Test_handleUpdateTask(t *testing.T) {
	var task fhir.Task
	taskData := mustReadFile("./testdata/task-update-accepted.json")
	require.NoError(t, json.Unmarshal(taskData, &task))

	var carePlanBundle fhir.Bundle
	carePlanBundleData := mustReadFile("./careteamservice/testdata/001-input.json")
	require.NoError(t, json.Unmarshal(carePlanBundleData, &carePlanBundle))

	var carePlan fhir.CarePlan
	err := json.Unmarshal(carePlanBundle.Entry[0].Resource, &carePlan)
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	fhirClient := mock.NewMockClient(ctrl)
	service := &Service{
		fhirClient: fhirClient,
		profile:    profile.Test(),
	}
	fhirClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result *fhir.Bundle, option ...fhirclient.Option) error {
		*result = carePlanBundle
		return nil
	}).AnyTimes()
	// mock for ?id=1
	fhirClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result *fhir.Bundle, option ...fhirclient.Option) error {
		result.Entry = []fhir.BundleEntry{
			{
				Resource: taskData,
			},
		}
		return nil
	}).AnyTimes()
	fhirClient.EXPECT().Read("Task/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result *fhir.Task, option ...fhirclient.Option) error {
		*result = task
		return nil
	}).AnyTimes()

	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal2)

	updateRequest := func(fn ...func(*fhir.Task)) FHIRHandlerRequest {
		updatedTask := deep.Copy(task)
		updatedTask.Status = fhir.TaskStatusInProgress
		for _, f := range fn {
			f(&updatedTask)
		}
		updatedTaskData, _ := json.Marshal(updatedTask)
		requestUrl, _ := url.Parse("Task/" + *task.Id)
		return FHIRHandlerRequest{
			ResourceData: updatedTaskData,
			ResourcePath: requestUrl.Path,
			ResourceId:   *updatedTask.Id,
			RequestUrl:   requestUrl,
			HttpMethod:   "PUT",
			Principal:    auth.TestPrincipal2,
			LocalIdentity: &fhir.Identifier{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  to.Ptr("2"),
			},
		}
	}

	t.Run("Task is identified by search parameters", func(t *testing.T) {
		request := updateRequest()
		request.RequestUrl, _ = url.Parse("Task?_id=" + *task.Id)
		request.ResourcePath = request.RequestUrl.Path
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.NoError(t, err)
		require.Len(t, tx.Entry, 4)
		require.Equal(t, "Task?_id=1", tx.Entry[0].Request.Url)
		require.Equal(t, fhir.HTTPVerbPUT, tx.Entry[0].Request.Method)
		audit.VerifyAuditEventForTest(t, &tx.Entry[1], "Task/"+*task.Id, fhir.AuditEventActionU, &fhir.Reference{
			Identifier: &auth.TestPrincipal2.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
	})
	t.Run("task status isn't updated", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.Status = fhir.TaskStatusAccepted
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.NoError(t, err)
		require.Len(t, tx.Entry, 4)
		require.Equal(t, fhir.HTTPVerbPUT, tx.Entry[0].Request.Method)
		audit.VerifyAuditEventForTest(t, &tx.Entry[1], "Task/"+*task.Id, fhir.AuditEventActionU, &fhir.Reference{
			Identifier: &auth.TestPrincipal2.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
	})
	t.Run("original FHIR request headers are passed to outgoing Bundle entry", func(t *testing.T) {
		request := updateRequest()
		request.HttpHeaders = map[string][]string{
			"If-None-Exist": {"ifnoneexist"},
		}
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.NoError(t, err)
		require.Equal(t, "ifnoneexist", *tx.Entry[0].Request.IfNoneExist)
	})
	t.Run("error: resource ID can't be changed (while Task is identified by search parameters)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.Id = to.Ptr("1000")
		})
		request.RequestUrl, _ = url.Parse("Task?_id=" + *task.Id)
		request.ResourceId = ""
		request.ResourcePath = request.RequestUrl.Path
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "ID in request URL does not match ID in resource")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: change Task.requester (not allowed)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.Requester = &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("attacker-ura"),
				},
			}
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "Task.requester cannot be changed")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: unsecure external literal reference", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.For = &fhir.Reference{
				Reference: to.Ptr("http://example.com/Patient/1"),
			}
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "literal reference is URL with scheme http://, only https:// is allowed (path=for.reference)")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: change Task.owner (not allowed)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.Owner = &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("attacker-ura"),
				},
			}
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "Task.owner cannot be changed")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: change Task.basedOn (not allowed)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.BasedOn = []fhir.Reference{
				{
					Reference: to.Ptr("Task/2"),
				},
			}
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "Task.basedOn cannot be changed")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: change Task.partOf (not allowed)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.PartOf = append(task.PartOf, fhir.Reference{
				Reference: to.Ptr("Task/2"),
			})
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "Task.partOf cannot be changed")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: request.ID != resource.ID", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.Id = to.Ptr("1000")
		})
		request.RequestUrl, _ = url.Parse("Task/1")
		request.ResourceId = "1"
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "ID in request URL does not match ID in resource")
		require.Empty(t, tx.Entry)
	})
	t.Run("error: change Task.for (not allowed)", func(t *testing.T) {
		request := updateRequest(func(task *fhir.Task) {
			task.For = &fhir.Reference{
				Reference: to.Ptr("Patient/2"),
			}
		})
		tx := coolfhir.Transaction()

		_, err := service.handleUpdateTask(ctx, request, tx)

		require.EqualError(t, err, "Task.for cannot be changed")
		require.Empty(t, tx.Entry)
	})
}

func Test_handleUpdateTask_Validation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
		profile:    profile.Test(),
	}

	updateTaskAcceptedData := mustReadFile("./testdata/task-update-accepted.json")

	tests := []struct {
		name          string
		ctx           context.Context
		request       FHIRHandlerRequest
		existingTask  *fhir.Task
		errorFromRead error
		wantErr       bool
	}{
		{
			name: "invalid task update - invalid JSON",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: []byte(`{"resourceType": "Task", "status":`),
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid task update - missing required fields",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: []byte(`{"resourceType": "Task"}`),
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid task update - invalid state transition",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: []byte(`{"resourceType": "Task", "status": "received", "intent":"order"}`),
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			existingTask: &fhir.Task{
				Id:     to.Ptr("1"),
				Status: fhir.TaskStatusInProgress,
				Intent: "order",
			},
			errorFromRead: nil,
			wantErr:       true,
		},
		{
			name: "valid task update - not authenticated",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: []byte(`{"resourceType": "Task", "status": "completed", "intent":"order"}`),
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			existingTask: &fhir.Task{
				Status: fhir.TaskStatusRequested,
			},
			errorFromRead: nil,
			wantErr:       true,
		},
		{
			name: "valid task update - error from read",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: []byte(`{"resourceType": "Task", "status": "completed", "intent":"order"}`),
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			existingTask: &fhir.Task{
				Status: fhir.TaskStatusRequested,
			},
			errorFromRead: errors.New("error"),
			wantErr:       true,
		},
		{
			name: "valid task update - not owner or requester",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateTaskAcceptedData,
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal3,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("3"),
				},
			},
			existingTask: &fhir.Task{
				Owner:  &fhir.Reference{Identifier: &auth.TestPrincipal2.Organization.Identifier[0]},
				Status: fhir.TaskStatusRequested,
			},
			errorFromRead: nil,
			wantErr:       true,
		},
		{
			name: "valid task update - requester, invalid state transition",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateTaskAcceptedData,
				ResourcePath: "Task/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr(coolfhir.URANamingSystem),
					Value:  to.Ptr("1"),
				},
			},
			existingTask: &fhir.Task{
				Owner:  &fhir.Reference{Identifier: &auth.TestPrincipal1.Organization.Identifier[0]},
				Status: fhir.TaskStatusRequested,
			},
			errorFromRead: nil,
			wantErr:       true,
		},
		// TODO: Positive test cases. These are complex to mock with the side effects of fhir.QueryParam, refactor unit tests to http tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &coolfhir.BundleBuilder{}

			if tt.existingTask != nil {
				mockFHIRClient.EXPECT().Read("Task/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.existingTask))
					return tt.errorFromRead
				})
			}

			_, err := service.handleUpdateTask(tt.ctx, tt.request, tx)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleUpdateTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
