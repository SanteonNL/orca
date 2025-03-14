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
	condition1Raw, _ := json.Marshal(condition1)

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
					Location: to.Ptr("Condition/1"),
					Status:   "200 OK",
				},
				Resource: condition1Raw,
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
		setup         func(ctx context.Context, client *mock.MockClient)
		principal     *auth.Principal
	}{
		"error: Condition does not exist": {
			principal:     auth.TestPrincipal1,
			expectedError: errors.New("fhir error: Condition not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "Condition/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Condition not found"))
			},
		},
		"error: Condition exists, no subject": {
			principal: auth.TestPrincipal1,
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
		"error: Condition exists, subject is patient, error fetching patient, incorrect principal": {
			principal: auth.TestPrincipal3,
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
					Return(errors.New("fhir error: Issues searching for patient")).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					}).AnyTimes()
			},
		},
		"error: Condition exists, subject is patient, patient returned, incorrect principal": {
			principal: auth.TestPrincipal3,
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
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					}).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: patient1Raw}}}
						return nil
					}).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					}).AnyTimes()
			},
		},
		"error: Condition exists, subject is patient, patient returned, incorrect principal, AuditEvent is found": {
			principal: auth.TestPrincipal3,
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
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					}).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: patient1Raw}}}
						return nil
					}).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					}).AnyTimes()
			},
		},
		"ok: Condition exists, subject is patient, no patient returned, is creator of condition": {
			principal: auth.TestPrincipal1,
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
				client.EXPECT().SearchWithContext(ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					}).AnyTimes()
			},
		},
		"ok: Condition exists, subject is patient, patient returned, correct principal": {
			principal: auth.TestPrincipal1,
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
					}).AnyTimes()
				client.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: carePlan1Raw}}}
						return nil
					}).AnyTimes()
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock.NewMockClient(ctrl)

			if tt.setup != nil {
				tt.setup(context.Background(), client)
			}

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}

			request := FHIRHandlerRequest{
				ResourceId:    "1",
				Principal:     tt.principal,
				LocalIdentity: &fhir.Identifier{System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"), Value: to.Ptr("1")},
			}
			result, err := service.handleGetCondition(context.Background(), request, tx)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				res, _, err := result(defaultReturnedBundle)
				require.NoError(t, err)
				require.JSONEq(t, string(condition1Raw), string(res.Resource))

				require.Equal(t, "Condition/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
			}
		})
	}
}
