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

	tests := map[string]struct {
		context       context.Context
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: Patient does not exist": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).Return(fhirclient.OperationOutcomeError{
					HttpStatusCode: http.StatusNotFound,
				})
			},
		},
		"error: Patient exists, auth, error fetching CarePlan": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			// returnedResource: &patient1,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Patient) = patient1
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
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Patient) = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"error: Patient exists, auth, CarePlan returned, not a participant": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Patient) = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					})
			},
		},
		"ok: Patient exists, auth, CarePlan returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Patient/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Patient) = patient1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
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
			patient, err := service.handleGetPatient(tt.context, "1", &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, patient)
			} else {
				require.NoError(t, err)
				require.NotNil(t, patient)
			}
		})
	}
}

func TestService_handleSearchPatient(t *testing.T) {
	careplan1Careteam2 := mustReadFile("./testdata/careplan1-careteam2.json")
	careplan2Careteam1 := mustReadFile("./testdata/careplan2-careteam1.json")
	patient1 := mustReadFile("./testdata/patient-1.json")
	patient2 := mustReadFile("./testdata/patient-2.json")

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
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"fhirclient error": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"Patient returned, error from CarePlan read": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("error"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("error"))
			},
		},
		"Patient returned, no careplan or careteam returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"Patient returned, careplan and careteam returned, incorrect principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: patient1},
						}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
						}}
						return nil
					})
			},
		},
		"Patient returned, careplan returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
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
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
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
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
						}}
						return nil
					})
			},
		},
		"Multiple resources returned, correctly filtered": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: patient1},
				},
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: patient1},
								{Resource: patient2},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{
							{Resource: careplan1Careteam2},
							{Resource: careplan2Careteam1},
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
			tt.setup(tt.context, client)

			service := &Service{fhirClient: client}
			bundle, err := service.handleSearchPatient(tt.context, url.Values{}, &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Equal(t, tt.expectedError, err)
				require.Nil(t, bundle)
			} else {
				require.Nil(t, err)
				require.Equal(t, tt.expectedBundle, bundle)
			}

			// testHelperHandleSearchResource[fhir.Patient](t, tt, service.handleSearchPatient)
		})
	}
}
