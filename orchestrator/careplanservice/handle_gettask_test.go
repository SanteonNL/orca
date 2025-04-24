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
				Principal:    auth.TestPrincipal3,
				ResourceId:   "1",
				ResourcePath: "Task/1",
				FhirHeaders:  &fhirclient.Headers{},
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
				Principal:    auth.TestPrincipal3,
				ResourceId:   "1",
				ResourcePath: "Task/1",
				FhirHeaders:  &fhirclient.Headers{},
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
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"_id": []string{"1"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: careplan read failed"))
			},
		},
		"error: Task exists, auth, CarePlan and CareTeam returned, not a participant": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal3,
				ResourceId:   "1",
				ResourcePath: "Task/1",
				FhirHeaders:  &fhirclient.Headers{},
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
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"_id": []string{"1"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: carePlan1Raw,
								},
							},
						}
						return nil
					})
			},
		},
		"ok: Task exists, auth, CarePlan and CareTeam returned, owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				ResourceId:   "1",
				ResourcePath: "Task/1",
				FhirHeaders:  &fhirclient.Headers{},
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

			handler := &FHIRReadOperationHandler[fhir.Task]{
				fhirClient:  client,
				authzPolicy: ReadTaskAuthzPolicy(client),
			}
			tx := coolfhir.Transaction()
			result, err := handler.Handle(tt.context, tt.request, tx)

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

				entries, notifications, err := result(mockResponse)
				require.NoError(t, err)
				require.NotNil(t, entries)
				var task fhir.Task
				err = json.Unmarshal(entries[0].Resource, &task)
				require.NoError(t, err)
				require.Equal(t, task.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchTask(t *testing.T) {
	var careplan fhir.CarePlan
	careplanRaw := mustReadFile("./testdata/careplan1-careteam2.json")
	_ = json.Unmarshal(careplanRaw, &careplan)

	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)

	task2Raw := mustReadFile("./testdata/task-2.json")
	var task2 fhir.Task
	_ = json.Unmarshal(task2Raw, &task2)

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Task/1"),
				},
			},
		},
	}
	auditEventReadRaw, _ := json.Marshal(auditEventRead)

	tests := map[string]struct {
		expectedError   error
		context         context.Context
		request         FHIRHandlerRequest
		setup           func(ctx context.Context, client *mock.MockClient)
		mockResponse    *fhir.Bundle
		expectedEntries []string
	}{
		"empty bundle": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				QueryParams:  url.Values{},
				ResourcePath: "Task",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{"_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					Return(nil)
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"task returned, auth, task owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				QueryParams:  url.Values{},
				ResourcePath: "Task",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{"_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}

						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: task1Raw,
					Response: &fhir.BundleEntryResponse{
						Status: "200 OK",
					},
				},
				{
					Resource: auditEventReadRaw,
					Response: &fhir.BundleEntryResponse{
						Status: "200 OK",
					},
				},
			}},
			expectedEntries: []string{string(task1Raw)},
		},
		"task returned, auth, not task owner, participant in CareTeam": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				QueryParams:  url.Values{},
				ResourcePath: "Task",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Task", url.Values{"_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task2Raw},
							},
						}

						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"_id": []string{"1"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: careplanRaw,
								},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: task2Raw,
					Response: &fhir.BundleEntryResponse{
						Status: "200 OK",
					},
				},
				{
					Resource: auditEventReadRaw,
					Response: &fhir.BundleEntryResponse{
						Status: "200 OK",
					},
				},
			}},
			expectedEntries: []string{string(task2Raw)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			handler := FHIRSearchOperationHandler[fhir.Task]{
				fhirClient:  client,
				authzPolicy: ReadTaskAuthzPolicy(client),
			}
			tx := coolfhir.Transaction()
			result, err := handler.Handle(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				entries, notifications, err := result(tt.mockResponse)
				require.NoError(t, err)
				require.NotNil(t, entries)
				require.Len(t, notifications, 0)

				if len(tt.mockResponse.Entry) > 0 {
					// We expect half the entries in the mock response, because we are returning the task and the audit event, but entries only has tasks
					require.Len(t, entries, len(tt.mockResponse.Entry)/2)

					actualEntries := []string{}
					for _, entry := range entries {
						var task fhir.Task
						err := json.Unmarshal(entry.Resource, &task)
						require.NoError(t, err)
						actualEntries = append(actualEntries, string(entry.Resource))
					}

					require.ElementsMatch(t, tt.expectedEntries, actualEntries)
				}
			}
		})
	}
}
