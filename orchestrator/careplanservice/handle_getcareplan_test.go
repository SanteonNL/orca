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

func TestService_handleGetCarePlan(t *testing.T) {
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	auditEvent := fhir.AuditEvent{
		Id: to.Ptr("1"),
	}
	auditEventRaw, _ := json.Marshal(auditEvent)

	tests := map[string]struct {
		expectedError error
		context       context.Context
		request       FHIRHandlerRequest
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: CarePlan does not exist": {
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
			expectedError: errors.New("careplan not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					Return(errors.New("careplan not found"))
			},
		},
		"error: CarePlan returned, incorrect principal": {
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
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.CarePlan, _ ...fhirclient.Option) error {
						*target = carePlan1
						return nil
					})
			},
		},
		"ok: CarePlan returned, correct principal": {
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
				client.EXPECT().ReadWithContext(ctx, "CarePlan/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.CarePlan, _ ...fhirclient.Option) error {
						*target = carePlan1
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
			if tt.setup != nil {
				tt.setup(tt.context, client)
			}

			service := &Service{fhirClient: client}
			tx := coolfhir.Transaction()
			result, err := service.handleReadCarePlan(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				mockResponse := &fhir.Bundle{
					Entry: []fhir.BundleEntry{
						{
							Resource: carePlan1Raw,
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
				var carePlan fhir.CarePlan
				err = json.Unmarshal(entries[0].Resource, &carePlan)
				require.NoError(t, err)
				require.Equal(t, carePlan.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchCarePlan(t *testing.T) {
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	carePlan2Raw := mustReadFile("./testdata/careplan2-careteam1.json")
	var carePlan2 fhir.CarePlan
	_ = json.Unmarshal(carePlan2Raw, &carePlan2)

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("CarePlan/1"),
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
				ResourcePath: "CarePlan/_search",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"careplan returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				QueryParams:  url.Values{},
				ResourcePath: "CarePlan/_search",
				FhirHeaders:  &fhirclient.Headers{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: carePlan1Raw},
							},
						}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: carePlan1Raw,
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
			expectedEntries: []string{string(carePlan1Raw)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			tt.setup(tt.context, client)

			handler := FHIRSearchOperationHandler[fhir.CarePlan]{
				fhirClient:  client,
				authzPolicy: ReadCarePlanAuthzPolicy(),
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
					// We expect half the entries in the mock response, because we are returning the patient and the audit event, but entries only has careplans
					require.Len(t, entries, len(tt.mockResponse.Entry)/2)

					actualEntries := []string{}
					for _, entry := range entries {
						var carePlan fhir.CarePlan
						err := json.Unmarshal(entry.Resource, &carePlan)
						require.NoError(t, err)
						actualEntries = append(actualEntries, string(entry.Resource))
					}

					require.ElementsMatch(t, tt.expectedEntries, actualEntries)
				}
			}
		})
	}
}
