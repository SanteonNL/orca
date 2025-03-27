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
			result, err := service.handleGetCarePlan(tt.context, tt.request, tx)

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

				entry, notifications, err := result(mockResponse)
				require.NoError(t, err)
				require.NotNil(t, entry)
				var carePlan fhir.CarePlan
				err = json.Unmarshal(entry.Resource, &carePlan)
				require.NoError(t, err)
				require.Equal(t, carePlan.Id, carePlan1.Id)

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchCarePlan(t *testing.T) {
	carePlan1 := mustReadFile("./testdata/careplan1-careteam2.json")
	carePlan2 := mustReadFile("./testdata/careplan2-careteam1.json")

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
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
		},
		"fhirclient error": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"CarePlan returned, incorrect principal": {
			context:        auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			expectedBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: carePlan1,
								},
							},
						}
						return nil
					})
			},
		},
		"CarePlan returned, correct principal": {
			context:        auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1}}},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: carePlan1},
							},
						}
						return nil
					})
			},
		},
		"Multiple CarePlans returned, correct principal, results filtered": {
			context:        auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1}}},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: carePlan1},
								{Resource: carePlan2},
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
			result, err := service.handleSearchCarePlan(tt.context, url.Values{}, &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Nil(t, result)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.NotNil(t, result)
				require.Equal(t, tt.expectedBundle, result)
				require.NoError(t, err)
			}
		})
	}
}
