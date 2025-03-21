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

func TestService_handleGetCondition(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	patient1Raw := mustReadFile("./testdata/patient-1.json")
	var patient1 fhir.Patient
	_ = json.Unmarshal(patient1Raw, &patient1)

	condition1 := fhir.Condition{
		Id: to.Ptr("1"),
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("123456789"),
			},
		},
	}

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
		expectedError error
		setup         func(ctx context.Context, client *mock.MockClient)
	}{
		"error: Condition does not exist": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: Condition not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Condition not found"))
			},
		},
		"error: Condition exists, no subject": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = fhir.Condition{Id: to.Ptr("1")}
						return nil
					})
			},
		},
		"error: Condition exists, subject is not a patient": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: Issues searching for patient"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = fhir.Condition{
							Id: to.Ptr("1"),
							Subject: fhir.Reference{
								Identifier: &fhir.Identifier{
									System: to.Ptr("SomethingWrong"),
									Value:  to.Ptr("1"),
								},
							},
						}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Issues searching for patient"))
			},
		},
		"error: Condition exists, subject is patient, error fetching patient": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: Issues searching for patient"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = condition1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Issues searching for patient"))
			},
		},
		"error: Condition exists, no patient returned": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = condition1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"error: Condition exists, subject is patient, patient returned, incorrect principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = condition1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: patient1Raw}}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					})
			},
		},
		"ok: Condition exists, subject is patient, patient returned, incorrect principal, but AuditEvent is found": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = condition1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: patient1Raw}}}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					})
			},
		},
		"ok: Condition exists, subject is patient, patient returned, correct principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target *fhir.Condition, _ ...fhirclient.Option) error {
						*target = condition1
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: patient1Raw}}}
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)

			if tt.setup != nil {
				tt.setup(tt.context, client)
			}

			service := &Service{fhirClient: client}
			condition, err := service.handleGetCondition(tt.context, "1", &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, condition)
			} else {
				require.NoError(t, err)
				require.NotNil(t, condition)
			}
		})
	}
}
