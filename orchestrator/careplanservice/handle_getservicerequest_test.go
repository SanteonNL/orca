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

	serviceRequest := fhir.ServiceRequest{
		Id: to.Ptr("1"),
	}
	serviceRequestRaw, _ := json.Marshal(serviceRequest)

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
					Location: to.Ptr("ServiceRequest/1"),
					Status:   "200 OK",
				},
				Resource: serviceRequestRaw,
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
		"error: ServiceRequest does not exist": {
			principal:     auth.TestPrincipal1,
			readError:     errors.New("fhir error: ServiceRequest not found"),
			expectedError: errors.New("fhir error: ServiceRequest not found"),
		},
		"error: ServiceRequest exists, error searching for task": {
			principal:     auth.TestPrincipal1,
			expectedError: errors.New("fhir error: Issue searching for task"),
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("fhir error: Issue searching for task"))
			},
		},
		"error: ServiceRequest exists, no tasks, not creator": {
			principal: auth.TestPrincipal3,
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to ServiceRequest",
				StatusCode: http.StatusForbidden,
			},
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "Task", gomock.Any(), gomock.Any(), gomock.Any()).
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
		"ok: ServiceRequest exists, task found": {
			principal: auth.TestPrincipal1,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "Task", gomock.Any(), gomock.Any(), gomock.Any()).
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
		"ok: ServiceRequest exists, no tasks, is creator": {
			principal: auth.TestPrincipal1,
			setup: func(ctx context.Context, client *mock.MockClient) {
				client.EXPECT().SearchWithContext(gomock.Any(), "Task", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{Entry: []fhir.BundleEntry{}}
						return nil
					})
				client.EXPECT().SearchWithContext(gomock.Any(), "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target *fhir.Bundle, _ ...fhirclient.Option) error {
						*target = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{
									Resource: auditEventRaw,
								},
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
			client.EXPECT().ReadWithContext(gomock.Any(), "ServiceRequest/1", gomock.Any()).DoAndReturn(func(_ context.Context, _ string, target any, _ ...fhirclient.Option) error {
				*target.(*fhir.ServiceRequest) = serviceRequest
				return tt.readError
			}).AnyTimes()

			if tt.setup != nil && tt.readError == nil {
				tt.setup(auth.WithPrincipal(context.Background(), *tt.principal), client)
			}

			tx := coolfhir.Transaction()
			service := &Service{fhirClient: client}
			result, err := service.handleGetServiceRequest(auth.WithPrincipal(context.Background(), *tt.principal), FHIRHandlerRequest{
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
				require.JSONEq(t, string(serviceRequestRaw), string(res.Resource))

				require.Len(t, tx.Entry, 2)
				require.Equal(t, "ServiceRequest/1", tx.Entry[0].Request.Url)
				require.Equal(t, fhir.HTTPVerbGET, tx.Entry[0].Request.Method)
				require.Equal(t, "AuditEvent", tx.Entry[1].Request.Url)
				require.Equal(t, fhir.HTTPVerbPOST, tx.Entry[1].Request.Method)
			}
		})
	}
}
