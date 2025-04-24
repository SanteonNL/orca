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
		"error: Patient exists, auth, error fetching CarePlan": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:    "1",
				ResourcePath:  "Patient/1",
				Principal:     auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{},
				FhirHeaders:   &fhirclient.Headers{},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Patient, _ ...fhirclient.Option) error {
						*target = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(fhirclient.OperationOutcomeError{
						HttpStatusCode: http.StatusNotFound,
					})
			},
		},
		"error: Patient exists, auth, No CarePlans returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:    "1",
				ResourcePath:  "Patient/1",
				Principal:     auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{},
				FhirHeaders:   &fhirclient.Headers{},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Patient, _ ...fhirclient.Option) error {
						*target = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"error: Patient exists, auth, CarePlan returned, not a participant": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				ResourceId:    "1",
				Principal:     auth.TestPrincipal3,
				LocalIdentity: &fhir.Identifier{},
				FhirHeaders:   &fhirclient.Headers{},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Patient, _ ...fhirclient.Option) error {
						*target = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					})
			},
		},
		"ok: Patient exists, auth, CarePlan returned, not a participant, is creator": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				ResourceId:    "1",
				Principal:     auth.TestPrincipal3,
				LocalIdentity: &fhir.Identifier{},
				FhirHeaders:   &fhirclient.Headers{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Patient, _ ...fhirclient.Option) error {
						*target = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					})
			},
		},
		"ok: Patient exists, auth, CarePlan returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:    "1",
				ResourcePath:  "Patient/1",
				Principal:     auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{},
				FhirHeaders:   &fhirclient.Headers{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Patient, _ ...fhirclient.Option) error {
						*target = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
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

			handler := &FHIRReadOperationHandler[fhir.Patient]{
				fhirClient:  client,
				authzPolicy: ReadPatientAuthzPolicy(client),
			}

			tx := coolfhir.Transaction()
			result, err := handler.Handle(tt.context, tt.request, tx)

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				mockResponse := &fhir.Bundle{
					Entry: []fhir.BundleEntry{
						{
							Resource: patient1Raw,
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
				var patient fhir.Patient
				err = json.Unmarshal(entries[0].Resource, &patient)
				require.NoError(t, err)
				require.Equal(t, patient.Id, to.Ptr("1"))

				require.Len(t, notifications, 0)
			}
		})
	}
}

func TestService_handleSearchPatient(t *testing.T) {
	careplan1Careteam2 := mustReadFile("./testdata/careplan1-careteam2.json")
	careplan2Careteam1 := mustReadFile("./testdata/careplan2-careteam1.json")
	careplan3Careteam3 := mustReadFile("./testdata/careplan3-careteam3.json")
	patient1 := mustReadFile("./testdata/patient-1.json")
	patient2 := mustReadFile("./testdata/patient-2.json")
	patient3 := mustReadFile("./testdata/patient-3.json")

	auditEventRead := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionR),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
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
		// TODO: Temporarily disabling the audit-based auth tests, re-enable tests once auth has been re-implemented
		shouldSkip bool
	}{
		"error: Empty bundle": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
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
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"error: Patient returned, error from CarePlan read": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
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
					Return(errors.New("error"))
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"error: Patient returned, no careplan or careteam returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
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
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"error: Patient returned, careplan and careteam returned, incorrect principal": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
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
			},
			mockResponse:    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedEntries: []string{},
		},
		"ok: Patient returned, careplan and careteam returned, incorrect principal, resource creator": {
			shouldSkip: true,
			context:    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			request: FHIRHandlerRequest{
				Principal:   auth.TestPrincipal3,
				FhirHeaders: &fhirclient.Headers{},
				RequestUrl:  &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
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
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: patient1,
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
			expectedEntries: []string{string(patient1)},
		},
		"ok: Patient returned, careplan returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				RequestUrl:   &url.URL{RawQuery: "_id=1"},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
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
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: patient1,
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
			expectedEntries: []string{string(patient1)},
		},
		"ok: Multiple resources returned, correctly filtered": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				Principal:    auth.TestPrincipal1,
				FhirHeaders:  &fhirclient.Headers{},
				ResourcePath: "Patient",
				QueryParams:  url.Values{"_id": []string{"1,2,3"}},
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			},
			expectedError: nil,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: patient1,
									Response: &fhir.BundleEntryResponse{
										Status: "200 OK",
									},
								},
								{
									Resource: patient2,
									Response: &fhir.BundleEntryResponse{
										Status: "200 OK",
									},
								},
								{
									Resource: patient3,
									Response: &fhir.BundleEntryResponse{
										Status: "200 OK",
									},
								},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"subject": []string{"Patient/1"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{
								Resource: careplan1Careteam2,
								Response: &fhir.BundleEntryResponse{
									Status: "200 OK",
								},
							},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"subject": []string{"Patient/2"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{
								Resource: careplan2Careteam1,
								Response: &fhir.BundleEntryResponse{
									Status: "200 OK",
								},
							},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", url.Values{"subject": []string{"Patient/3"}, "_count": []string{"10000"}}, gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{
							{
								Resource: careplan3Careteam3,
								Response: &fhir.BundleEntryResponse{
									Status: "200 OK",
								},
							},
						}}
						return nil
					})
			},
			mockResponse: &fhir.Bundle{Entry: []fhir.BundleEntry{
				{
					Resource: patient1,
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
					Resource: patient3,
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
			expectedEntries: []string{string(patient1), string(patient3)},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)

			if tt.shouldSkip {
				t.Skip()
			}

			tt.setup(tt.context, client)

			handler := &FHIRSearchOperationHandler[fhir.Patient]{
				fhirClient:  client,
				authzPolicy: ReadPatientAuthzPolicy(client),
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
					// We expect half the entries in the mock response, because we are returning the patient and the audit event, but entries only has patients
					require.Len(t, entries, len(tt.mockResponse.Entry)/2)

					actualEntries := []string{}
					for _, entry := range entries {
						var patient fhir.Patient
						err := json.Unmarshal(entry.Resource, &patient)
						require.NoError(t, err)
						actualEntries = append(actualEntries, string(entry.Resource))
					}

					require.ElementsMatch(t, tt.expectedEntries, actualEntries)
				}
			}
		})
	}
}
