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

	var auditEventRaw []byte
	auditEventRaw, _ = json.Marshal(fhir.AuditEvent{
		Id: to.Ptr("2"),
	})

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Task/1"),
					Status:   "200 OK",
				},
				Resource: task1Raw,
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/2"),
					Status:   "200 OK",
				},
				Resource: auditEventRaw,
			},
		},
	}

	tests := map[string]struct {
		expectedError error
		readError     error
		principal     *auth.Principal
	}{
		"error: Task does not exist": {
			principal:     auth.TestPrincipal3,
			readError:     errors.New("fhir error: task not found"),
			expectedError: errors.New("fhir error: task not found"),
		},
		"error: Task exists, auth, not owner or requester, error fetching CarePlan": {
			principal:     auth.TestPrincipal3,
			readError:     nil,
			expectedError: errors.New("fhir error: careplan read failed"),
		},
		"error: Task exists, auth, CarePlan and CareTeam returned, not a participant": {
			principal: auth.TestPrincipal3,
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant is not part of CareTeam",
				StatusCode: http.StatusForbidden,
			},
		},
		"ok: Task exists, auth, CarePlan and CareTeam returned, owner": {
			principal: auth.TestPrincipal1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			client.EXPECT().ReadWithContext(gomock.Any(), "Task/1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
				*target.(*fhir.Task) = task1
				return tt.readError
			}).AnyTimes()

			if tt.readError == nil && tt.principal == auth.TestPrincipal3 {
				if tt.expectedError != nil && tt.expectedError.Error() == "fhir error: careplan read failed" {
					client.EXPECT().ReadWithContext(gomock.Any(), "CarePlan/1", gomock.Any()).
						Return(errors.New("fhir error: careplan read failed"))
				} else {
					client.EXPECT().ReadWithContext(gomock.Any(), "CarePlan/1", gomock.Any()).
						DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
							*target.(*fhir.CarePlan) = carePlan1
							return nil
						})
				}
			}

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}
			result, err := service.handleGetTask(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
				ResourceId: "1",
				Principal:  tt.principal,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			}, tx)

			if tt.expectedError != nil {
				require.Len(t, tx.Entry, 0)
				require.Equal(t, tt.expectedError, err)
			} else {
				res, _, err := result(defaultReturnedBundle)
				require.NoError(t, err)
				require.JSONEq(t, string(task1Raw), string(res.Resource))

				require.Len(t, tx.Entry, 2)
				require.Equal(t, "Task/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
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
