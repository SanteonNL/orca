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

func TestService_handleGetPatient(t *testing.T) {
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	patient1Raw := mustReadFile("./testdata/patient-1.json")
	var patient1 fhir.Patient
	_ = json.Unmarshal(patient1Raw, &patient1)

	auditEvent := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionC),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
				},
			},
		},
		Agent: []fhir.AuditEventAgent{
			{
				Who: &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
						Value:  to.Ptr("1"),
					},
				},
			},
		},
	}
	auditEventRaw, _ := json.Marshal(auditEvent)

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Patient/1"),
					Status:   "200 OK",
				},
				Resource: patient1Raw,
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
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: Patient does not exist": {
			principal: auth.TestPrincipal1,
			expectedError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			readError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
		},
		"error: Patient exists, no authorized patients": {
			principal: auth.TestPrincipal3,
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
				client.EXPECT().SearchWithContext(gomock.Any(), "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"ok: Patient exists, authorized via CarePlan": {
			principal: auth.TestPrincipal1,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: carePlan1Raw},
						}}
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
			client.EXPECT().ReadWithContext(gomock.Any(), "Patient/1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
				*target.(*fhir.Patient) = patient1
				return tt.readError
			}).AnyTimes()

			if tt.setup != nil && tt.readError == nil {
				tt.setup(auth.WithPrincipal(context.Background(), *tt.principal), client)
			}

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}
			result, err := service.handleGetPatient(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
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
				require.JSONEq(t, string(patient1Raw), string(res.Resource))

				require.Len(t, tx.Entry, 2)
				require.Equal(t, "Patient/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
			}
		})
	}
}

func TestService_handleSearchPatient(t *testing.T) {
	careplan1Careteam2 := mustReadFile("./testdata/careplan1-careteam2.json")
	careplan2Careteam1 := mustReadFile("./testdata/careplan2-careteam1.json")
	patient1 := mustReadFile("./testdata/patient-1.json")
	patient2 := mustReadFile("./testdata/patient-2.json")

	auditEvent := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionC),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
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
		expectedError  error
		expectedBundle *fhir.Bundle
		setup          func(ctx context.Context, client *mock.MockClient)
		principal      *auth.Principal
	}{
		"error: Empty bundle": {
			principal: auth.TestPrincipal1,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"error: fhirclient error": {
			principal:     auth.TestPrincipal1,
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"error: Patient returned, error from CarePlan read": {
			principal:     auth.TestPrincipal1,
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"error: Patient returned, no careplan or careteam returned": {
			principal:      auth.TestPrincipal1,
			expectedBundle: &fhir.Bundle{},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						return nil
					})
			},
		},
		"error: Patient returned, careplan and careteam returned, incorrect principal": {
			principal: auth.TestPrincipal3,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						return nil
					})
			},
		},
		"ok: Patient returned, careplan and careteam returned, incorrect principal, resource creator": {
			principal: auth.TestPrincipal3,
			expectedBundle: &fhir.Bundle{
				Link: []fhir.BundleLink{
					{
						Relation: "self",
						Url:      "http://example.com/fhir/Patient?some-query-params",
					},
				},
				Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Link: []fhir.BundleLink{
								{
									Relation: "self",
									Url:      "http://example.com/fhir/Patient?some-query-params",
								},
							},
							Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
							Entry: []fhir.BundleEntry{
								{Resource: patient1},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					})
			},
		},
		"ok: Patient returned, careplan returned, correct principal": {
			principal: auth.TestPrincipal1,
			expectedBundle: &fhir.Bundle{
				Link: []fhir.BundleLink{
					{
						Relation: "self",
						Url:      "http://example.com/fhir/Patient?some-query-params",
					},
				},
				Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Link: []fhir.BundleLink{
								{
									Relation: "self",
									Url:      "http://example.com/fhir/Patient?some-query-params",
								},
							},
							Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
							Entry: []fhir.BundleEntry{
								{Resource: patient1},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
						}}
						return nil
					})
			},
		},
		"ok: Multiple resources returned, correctly filtered": {
			principal: auth.TestPrincipal1,
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: patient1},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: patient1},
								{Resource: patient2},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
							{Resource: careplan2Careteam1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
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
			tt.setup(auth.WithPrincipal(context.Background(), *tt.principal), client)

			service := &Service{fhirClient: client}
			tx := coolfhir.Transaction()
			result, err := service.handleSearchPatient(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
				QueryParams: url.Values{},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
				Principal:    tt.principal,
				ResourcePath: "Patient",
			}, tx)

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				res, _, err := result(tt.expectedBundle)
				require.NoError(t, err)
				if len(tt.expectedBundle.Entry) > 0 {
					require.NotNil(t, res)

					require.Len(t, tx.Entry, len(tt.expectedBundle.Entry))
				}
			}
		})
	}
}
