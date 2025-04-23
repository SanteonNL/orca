package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
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

func TestService_handleGetQuestionnaire(t *testing.T) {
	questionnaire1 := fhir.Questionnaire{
		Id: to.Ptr("1"),
	}
	questionnaire1Raw, _ := json.Marshal(questionnaire1)
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
		"error: Questionnaire does not exist": {
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
			expectedError: errors.New("fhir error: Questionnaire not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Questionnaire/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Questionnaire not found"))
			},
		},
		"ok: Questionnaire exists, auth": {
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
				client.EXPECT().ReadWithContext(ctx, "Questionnaire/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Questionnaire, _ ...fhirclient.Option) error {
						*target = questionnaire1
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
			result, err := service.handleReadQuestionnaire(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				mockResponse := &fhir.Bundle{
					Entry: []fhir.BundleEntry{
						{
							Resource: questionnaire1Raw,
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
				var questionnaire fhir.Questionnaire
				err = json.Unmarshal(entries[0].Resource, &questionnaire)
				require.NoError(t, err)
				require.Equal(t, questionnaire.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchQuestionnaire(t *testing.T) {
	questionnaire1 := fhir.Questionnaire{
		Id:     to.Ptr("1"),
		Status: fhir.PublicationStatusActive,
		Title:  to.Ptr("Questionnaire 1"),
	}
	questionnaire1Raw, _ := json.Marshal(questionnaire1)

	questionnaire2 := fhir.Questionnaire{
		Id:     to.Ptr("2"),
		Status: fhir.PublicationStatusDraft,
		Title:  to.Ptr("Questionnaire 2"),
	}
	questionnaire2Raw, _ := json.Marshal(questionnaire2)

	questionnaire3 := fhir.Questionnaire{
		Id:     to.Ptr("3"),
		Status: fhir.PublicationStatusActive,
		Title:  to.Ptr("Questionnaire 3"),
	}
	questionnaire3Raw, _ := json.Marshal(questionnaire3)

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Questionnaire/1"),
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
		"empty bundle": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Questionnaire",
				RequestUrl:   &url.URL{RawQuery: "_id=nonexistent"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"fhirclient error": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Questionnaire",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"single questionnaire returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Questionnaire",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaire1Raw},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: questionnaire1Raw,
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
			expectedEntries: []string{string(questionnaire1Raw)},
		},
		"multiple questionnaires returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Questionnaire",
				QueryParams:  url.Values{"status": []string{"active"}},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "active", params.Get("status"))
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaire1Raw},
								{Resource: questionnaire3Raw},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: questionnaire1Raw,
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
					Resource: questionnaire3Raw,
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
			expectedEntries: []string{string(questionnaire1Raw), string(questionnaire3Raw)},
		},
		"search by multiple parameters": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Questionnaire",
				QueryParams:  url.Values{"_id": []string{"1,2,3"}, "status": []string{"draft"}},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, params url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						require.Equal(t, "1,2,3", params.Get("_id"))
						require.Equal(t, "draft", params.Get("status"))
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: questionnaire2Raw},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: questionnaire2Raw,
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
			expectedEntries: []string{string(questionnaire2Raw)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			handler := FHIRSearchOperationHandler{
				fhirClient: client,
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
