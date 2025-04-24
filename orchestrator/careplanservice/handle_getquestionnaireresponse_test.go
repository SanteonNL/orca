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

func TestService_handleGetQuestionnaireResponse(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw := mustReadFile("./testdata/careplan2-careteam1.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	questionnaireResponse1 := fhir.QuestionnaireResponse{
		Id: to.Ptr("1"),
	}
	questionnaireResponse1Raw, _ := json.Marshal(questionnaireResponse1)

	auditEvent := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionC),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("QuestionnaireResponse/1"),
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
		"error: QuestionnaireResponse exists, error fetching task": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				ResourceId:   "1",
				ResourcePath: "QuestionnaireResponse/1",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: errors.New("fhir error: no response"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.QuestionnaireResponse, _ ...fhirclient.Option) error {
						*target = questionnaireResponse1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: no response"))
			},
		},
		"error: QuestionnaireResponse exists, fetched task, incorrect principal (not task owner or requester)": {
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
				Message:    "Participant does not have access to QuestionnaireResponse",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.QuestionnaireResponse, _ ...fhirclient.Option) error {
						*target = questionnaireResponse1
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
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						return nil
					})
			},
		},
		"ok: QuestionnaireResponse exists, fetched task, incorrect principal, is creator": {
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
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.QuestionnaireResponse, _ ...fhirclient.Option) error {
						*target = questionnaireResponse1
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
		"ok: QuestionnaireResponse exists, fetched task, task owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				ResourceId:   "1",
				ResourcePath: "QuestionnaireResponse/1",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.QuestionnaireResponse, _ ...fhirclient.Option) error {
						*target = questionnaireResponse1
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

			handler := &FHIRReadOperationHandler[fhir.QuestionnaireResponse]{
				fhirClient:  client,
				authzPolicy: ReadQuestionnaireResponseAuthzPolicy(client),
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
							Resource: questionnaireResponse1Raw,
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
				var questionnaireResponse fhir.QuestionnaireResponse
				err = json.Unmarshal(entries[0].Resource, &questionnaireResponse)
				require.NoError(t, err)
				require.Equal(t, questionnaireResponse.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchQuestionnaireResponse(t *testing.T) {
	questionnaireResponse1 := fhir.QuestionnaireResponse{
		Id:     to.Ptr("1"),
		Status: fhir.QuestionnaireResponseStatusCompleted,
	}
	questionnaireResponse1Raw, _ := json.Marshal(questionnaireResponse1)

	questionnaireResponse2 := fhir.QuestionnaireResponse{
		Id:     to.Ptr("2"),
		Status: fhir.QuestionnaireResponseStatusInProgress,
	}
	questionnaireResponse2Raw, _ := json.Marshal(questionnaireResponse2)

	questionnaireResponse3 := fhir.QuestionnaireResponse{
		Id:     to.Ptr("3"),
		Status: fhir.QuestionnaireResponseStatusCompleted,
	}
	questionnaireResponse3Raw, _ := json.Marshal(questionnaireResponse3)

	task1 := fhir.Task{
		Id: to.Ptr("1"),
		Output: []fhir.TaskOutput{
			{
				ValueReference: &fhir.Reference{
					Reference: to.Ptr("QuestionnaireResponse/1"),
				},
			},
		},
	}
	task1Raw, _ := json.Marshal(task1)

	task3 := fhir.Task{
		Id: to.Ptr("3"),
		Output: []fhir.TaskOutput{
			{
				ValueReference: &fhir.Reference{
					Reference: to.Ptr("QuestionnaireResponse/3"),
				},
			},
		},
	}
	task3Raw, _ := json.Marshal(task3)

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("QuestionnaireResponse/1"),
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
		"found single QuestionnaireResponse with task access": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "QuestionnaireResponse",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "QuestionnaireResponse", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaireResponse1Raw},
							},
						}
						return nil
					})

				// Search for task referencing QuestionnaireResponse/1
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "QuestionnaireResponse/1", params.Get("output-reference"))
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
					Resource: questionnaireResponse1Raw,
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
			expectedEntries: []string{string(questionnaireResponse1Raw)},
		},
		"found QuestionnaireResponse as creator": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "QuestionnaireResponse",
				RequestUrl:   &url.URL{RawQuery: "_id=2"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "QuestionnaireResponse", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaireResponse2Raw},
							},
						}
						return nil
					})

				// Task search for QuestionnaireResponse/2 returns empty
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "QuestionnaireResponse/2", params.Get("output-reference"))
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})

				// isCreator check returns true
				// Because our implementation of isCreatorOfResource currently just returns true, we don't need to mock AuditEvent search
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: questionnaireResponse2Raw,
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
			expectedEntries: []string{string(questionnaireResponse2Raw)},
		},
		"multiple responses filtered correctly": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "QuestionnaireResponse",
				QueryParams:  url.Values{"status": {"completed"}},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "QuestionnaireResponse", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "completed", params.Get("status"))
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaireResponse1Raw},
								{Resource: questionnaireResponse3Raw},
							},
						}
						return nil
					})

				// Task search for each QuestionnaireResponse
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						if params.Get("output-reference") == "QuestionnaireResponse/1" {
							*target = fhir.Bundle{
								Entry: []fhir.BundleEntry{
									{Resource: task1Raw},
								},
							}
						} else if params.Get("output-reference") == "QuestionnaireResponse/3" {
							*target = fhir.Bundle{
								Entry: []fhir.BundleEntry{
									{Resource: task3Raw},
								},
							}
						}
						return nil
					}).Times(2)
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: questionnaireResponse1Raw,
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
					Resource: questionnaireResponse3Raw,
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
			expectedEntries: []string{string(questionnaireResponse1Raw), string(questionnaireResponse3Raw)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			handler := FHIRSearchOperationHandler[fhir.QuestionnaireResponse]{
				fhirClient:  client,
				authzPolicy: ReadQuestionnaireResponseAuthzPolicy(client),
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
