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

	var auditEventRaw []byte
	auditEventRaw, _ = json.Marshal(fhir.AuditEvent{
		Id: to.Ptr("2"),
	})

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("CarePlan/1"),
					Status:   "200 OK",
				},
				Resource: carePlan1Raw,
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
		"error: CarePlan does not exist": {
			principal:     auth.TestPrincipal1,
			readError:     errors.New("careplan not found"),
			expectedError: errors.New("careplan not found"),
		},
		"error: CarePlan returned, incorrect principal": {
			principal: auth.TestPrincipal3,
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant is not part of CareTeam",
				StatusCode: http.StatusForbidden,
			},
		},
		"ok: CarePlan returned, correct principal": {
			principal: auth.TestPrincipal1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)
			client.EXPECT().ReadWithContext(gomock.Any(), "CarePlan/1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
				*target.(*fhir.CarePlan) = carePlan1
				return tt.readError
			})

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}
			result, err := service.handleGetCarePlan(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
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
				require.JSONEq(t, string(carePlan1Raw), string(res.Resource))
				//require.Len(t, notifications, 2)
				//require.IsType(t, &fhir.CarePlan{}, notifications[0])
				//require.IsType(t, &fhir.AuditEvent{}, notifications[1])

				require.Len(t, tx.Entry, 2)
				require.Equal(t, "CarePlan/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
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
