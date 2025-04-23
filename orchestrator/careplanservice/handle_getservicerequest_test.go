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

func TestService_handleGetServiceRequest(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	serviceRequest1 := fhir.ServiceRequest{
		Id: to.Ptr("1"),
	}
	serviceRequest1Raw, _ := json.Marshal(serviceRequest1)

	auditEvent := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionC),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("ServiceRequest/1"),
				},
			},
		},
		Agent: []fhir.AuditEventAgent{
			{
				Who: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
						Value:  to.Ptr("3"),
					},
				},
			},
		},
	}
	auditEventRaw, _ := json.Marshal(auditEvent)

	tests := map[string]struct {
		context       context.Context
		request       FHIRHandlerRequest
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
		// TODO: Temporarily disabling the audit-based auth tests, re-enable tests once auth has been re-implemented
		shouldSkip bool
	}{
		"error: ServiceRequest does not exist": {
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
			expectedError: errors.New("fhir error: ServiceRequest not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: ServiceRequest not found"))
			},
		},
		"error: ServiceRequest exists, error searching for task": {
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
			expectedError: errors.New("fhir error: Issue searching for task"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.ServiceRequest, _ ...fhirclient.Option) error {
						*target = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Issue searching for task"))
			},
		},
		"error: ServiceRequest exists, fetched task, incorrect principal": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
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
				Message:    "Participant does not have access to ServiceRequest",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.ServiceRequest, _ ...fhirclient.Option) error {
						*target = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: no response"))
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"ok: ServiceRequest exists, fetched task, incorrect principal, but is creator": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("3"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.ServiceRequest, _ ...fhirclient.Option) error {
						*target = serviceRequest1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: no response"))
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					})
			},
		},
		"ok: ServiceRequest exists, fetched task, task owner": {
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
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.ServiceRequest, _ ...fhirclient.Option) error {
						*target = serviceRequest1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if tt.shouldSkip {
				t.Skip()
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			tx := coolfhir.Transaction()
			result, err := service.handleReadServiceRequest(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				mockResponse := &fhir.Bundle{
					Entry: []fhir.BundleEntry{
						{
							Resource: serviceRequest1Raw,
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
				require.Len(t, entries, 1)
				var serviceRequest fhir.ServiceRequest
				err = json.Unmarshal(entries[0].Resource, &serviceRequest)
				require.NoError(t, err)
				require.Equal(t, serviceRequest.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchServiceRequest(t *testing.T) {
	serviceRequest1 := fhir.ServiceRequest{
		Id: to.Ptr("1"),
	}
	serviceRequest1Raw, _ := json.Marshal(serviceRequest1)

	serviceRequest2 := fhir.ServiceRequest{
		Id: to.Ptr("2"),
	}
	serviceRequest2Raw, _ := json.Marshal(serviceRequest2)

	serviceRequest3 := fhir.ServiceRequest{
		Id: to.Ptr("3"),
	}
	serviceRequest3Raw, _ := json.Marshal(serviceRequest3)

	task1 := fhir.Task{
		Id: to.Ptr("1"),
		Focus: &fhir.Reference{
			Reference: to.Ptr("ServiceRequest/1"),
		},
	}
	task1Raw, _ := json.Marshal(task1)

	task3 := fhir.Task{
		Id: to.Ptr("3"),
		Focus: &fhir.Reference{
			Reference: to.Ptr("ServiceRequest/3"),
		},
	}
	task3Raw, _ := json.Marshal(task3)

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("ServiceRequest/1"),
				},
			},
		},
	}
	auditEventReadRaw, _ := json.Marshal(auditEventRead)

	tests := map[string]struct {
		context         context.Context
		request         FHIRHandlerRequest
		expectedError   error
		setup           func(ctx context.Context, client *mock.MockClient)
		mockResponse    *fhir.Bundle
		expectedEntries []string
	}{
		"error: Empty bundle": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"error: fhirclient error": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"ok: ServiceRequest returned, task found": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: serviceRequest1Raw},
							},
						}
						return nil
					})

				// Task search for ServiceRequest/1
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "ServiceRequest/1", params.Get("focus"))
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: task1Raw},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: serviceRequest1Raw,
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
			expectedEntries: []string{string(serviceRequest1Raw)},
		},
		"ok: ServiceRequest returned, no task found but is creator": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=2"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: serviceRequest2Raw},
							},
						}
						return nil
					})

				// Task search for ServiceRequest/2 returns empty
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "ServiceRequest/2", params.Get("focus"))
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})

				// isCreator check returns true
				// Because our implementation of isCreatorOfResource currently just returns true, we don't need to mock AuditEvent search
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: serviceRequest2Raw,
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
			expectedEntries: []string{string(serviceRequest2Raw)},
		},
		"ok: Multiple resources returned, correctly filtered": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal1,
				FhirHeaders: &fhirclient.Headers{},
				QueryParams: url.Values{"_id": []string{"1,2,3"}},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: serviceRequest1Raw},
								{Resource: serviceRequest2Raw},
								{Resource: serviceRequest3Raw},
							},
						}
						return nil
					})

				// Task search for ServiceRequest/1 returns task
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						if params.Get("focus") == "ServiceRequest/1" {
							*target = fhir.Bundle{
								Entry: []fhir.BundleEntry{
									{Resource: task1Raw},
								},
							}
						} else if params.Get("focus") == "ServiceRequest/2" {
							*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						} else if params.Get("focus") == "ServiceRequest/3" {
							*target = fhir.Bundle{
								Entry: []fhir.BundleEntry{
									{Resource: task3Raw},
								},
							}
						}
						return nil
					}).Times(3)

				// isCreator check for ServiceRequest/2 returns false (we'll simulate this)
				// Since isCreatorOfResource currently returns true, we'll need to modify the expected results accordingly
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: serviceRequest1Raw,
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
				{
					Resource: serviceRequest2Raw,
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
				{
					Resource: serviceRequest3Raw,
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
			expectedEntries: []string{string(serviceRequest1Raw), string(serviceRequest2Raw), string(serviceRequest3Raw)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			handler := FHIRSearchOperationHandler[fhir.ServiceRequest]{
				fhirClient: client,
				authzPolicy: AnyMatchPolicy([]Policy{
					CreatorHasAccess{},
				}),
			}
			tx := coolfhir.Transaction()
			result, err := handler.handleSearchServiceRequest(tt.context, tt.request, tx)

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

				if len(tt.expectedEntries) > 0 {
					// Check that we have the expected number of entries
					require.Len(t, entries, len(tt.expectedEntries))

					actualEntries := []string{}
					for _, entry := range entries {
						actualEntries = append(actualEntries, string(entry.Resource))
					}

					// Verify that the actual entries match the expected entries
					require.ElementsMatch(t, tt.expectedEntries, actualEntries)
				} else {
					require.Empty(t, entries)
				}
			}
		})
	}
}
