package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleGetTask(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	auditEvent := fhir.AuditEvent{
		Id: to.Ptr("1"),
	}
	auditEventRaw, _ := json.Marshal(auditEvent)

	tests := map[string]struct {
		context       context.Context
		request       FHIRHandlerRequest
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: Task does not exist": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("3"),
				},
			},
			expectedError: errors.New("fhir error: task not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Task/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: task not found"))
			},
		},
		"error: Task exists, auth, not owner or requester, error fetching CarePlan": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("3"),
				},
			},
			expectedError: errors.New("fhir error: careplan read failed"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Task/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Task, _ ...fhirclient.Option) error {
						*target = task1
						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: careplan read failed"))
			},
		},
		"error: Task exists, auth, CarePlan and CareTeam returned, not a participant": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("3"),
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant is not part of CareTeam",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Task/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Task, _ ...fhirclient.Option) error {
						*target = task1
						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.CarePlan, _ ...fhirclient.Option) error {
						*target = carePlan1
						return nil
					})
			},
		},
		"ok: Task exists, auth, CarePlan and CareTeam returned, owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Task/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Task, _ ...fhirclient.Option) error {
						*target = task1
						return nil
					})
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			tx := coolfhir.Transaction()
			result, err := service.handleGetTask(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				mockResponse := &fhir.Bundle{
					Entry: []fhir.BundleEntry{
						{
							Resource: task1Raw,
							Response: &fhir.BundleEntryResponse{
								Status: "200 OK",
							},
						},
						{
							Resource: auditEventRaw,
							Response: &fhir.BundleEntryResponse{
								Status: "200 OK",
							},
						},
					},
				}

				entry, notifications, err := result(mockResponse)
				require.NoError(t, err)
				require.NotNil(t, entry)
				var task fhir.Task
				err = json.Unmarshal(entry.Resource, &task)
				require.NoError(t, err)
				require.Equal(t, task.Id, task1.Id)

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchTask(t *testing.T) {
	var careplan fhir.CarePlan
	careplanRaw := mustReadFile("./testdata/careplan1-careteam2.json")
	_ = json.Unmarshal(careplanRaw, &careplan)

	task1 := mustReadFile("./testdata/task-1.json")
	task2 := mustReadFile("./testdata/task-2.json")

	tests := map[string]struct {
		context        context.Context
		expectedError  error
		expectedBundle *fhir.Bundle
		setup          func(ctx context.Context, client *mock.MockClient)
	}{
		"No auth": {
			context:       context.Background(),
			expectedError: errors.New("not authenticated"),
			setup:         func(ctx context.Context, client *mock.MockClient) {},
		},
		"Empty bundle": {
			context:        auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
		"fhirclient error - task": {
			context:        auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedError:  errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"Task returned, auth, task owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1,
					},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1},
							},
						}

						return nil
					})
			},
		},
		"Task returned, auth, not task owner, error from careplan read": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task2},
							},
						}

						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"Task returned, auth, not task owner, participant in CareTeam": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task2},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task2},
							},
						}

						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.CarePlan, _ ...fhirclient.Option) error {
						*target = careplan
						return nil
					})
			},
		},
		"Task returned, auth, not task owner, participant not in CareTeam": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task2},
							},
						}

						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.CarePlan, _ ...fhirclient.Option) error {
						*target = careplan
						return nil
					})
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			bundle, err := service.handleSearchTask(tt.context, url.Values{}, &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Nil(t, bundle)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.Equal(t, tt.expectedBundle, bundle)
				require.NoError(t, err)
			}
		})
	}
}
