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
	}{
		"error: QuestionnaireResponse does not exist": {
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
			expectedError: errors.New("fhir error: QuestionnaireResponse not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: QuestionnaireResponse not found"))
			},
		},
		"error: QuestionnaireResponse exists, error fetching task": {
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
		"error: QuestionnaireResponse exists, fetched task, incorrect principal (not task onwer or requester)": {
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
				Principal:   auth.TestPrincipal1,
				ResourceId:  "1",
				FhirHeaders: &fhirclient.Headers{},
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			tx := coolfhir.Transaction()
			result, err := service.handleGetQuestionnaireResponse(tt.context, tt.request, tx)

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
