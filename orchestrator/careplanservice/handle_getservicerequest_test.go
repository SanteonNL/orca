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

func TestService_handleGetServiceRequest(t *testing.T) {
	task1Raw := mustReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)

	auditEvent := fhir.AuditEvent{
		Id:     to.Ptr("1"),
		Action: to.Ptr(fhir.AuditEventActionC),
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("ServiceRequest/1"),
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
		"error: ServiceRequest does not exist": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: ServiceRequest not found"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: ServiceRequest not found"))
			},
		},
		"error: ServiceRequest exists, error searching for task": {
			context:       auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			expectedError: errors.New("fhir error: Issue searching for task"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.ServiceRequest) = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Issue searching for task"))
			},
		},
		"error: ServiceRequest exists, fetched task, incorrect principal": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "No creation audit event found for ServiceRequest",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.ServiceRequest) = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
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
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
			},
		},
		"ok: ServiceRequest exists, fetched task, incorrect principal, but is creator": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.ServiceRequest) = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
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
						*target.(*fhir.Bundle) = fhir.Bundle{Entry: []fhir.BundleEntry{{Resource: auditEventRaw}}}
						return nil
					})
			},
		},
		"ok: ServiceRequest exists, fetched task, task owner": {
			context: auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().ReadWithContext(ctx, "ServiceRequest/1", gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.ServiceRequest) = fhir.ServiceRequest{Id: to.Ptr("1")}
						return nil
					})
				client.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
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
			serviceRequest, err := service.handleGetServiceRequest(tt.context, "1", &fhirclient.Headers{})

			if tt.expectedError != nil {
				require.Nil(t, serviceRequest)
				require.Equal(t, tt.expectedError, err)
			} else {
				require.NoError(t, err)
				require.NotNil(t, serviceRequest)
			}
		})
	}
}
