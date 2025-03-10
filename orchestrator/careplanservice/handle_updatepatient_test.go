package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func Test_handleUpdatePatient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultPatient := fhir.Patient{
		Id: to.Ptr("1"),
		Name: []fhir.HumanName{
			{
				Given:  []string{"Jan"},
				Family: to.Ptr("Smit"),
			},
		},
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}

	updatePatientData, _ := json.Marshal(defaultPatient)

	// Create a mock audit event for the creation
	creationAuditEvent := fhir.AuditEvent{
		Id: to.Ptr("audit1"),
		Agent: []fhir.AuditEventAgent{
			{
				Who: &fhir.Reference{
					Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
				},
			},
		},
		Entity: []fhir.AuditEventEntity{
			{
				What: &fhir.Reference{
					Reference: to.Ptr("Patient/1"),
				},
			},
		},
	}

	creationAuditBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(creationAuditEvent)
					return b
				}(),
			},
		},
	}

	existingPatientBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultPatient)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                  string
		ctx                   context.Context
		request               FHIRHandlerRequest
		existingPatientBundle *fhir.Bundle
		errorFromSearch       error
		errorFromAuditQuery   error
		auditBundle           *fhir.Bundle
		wantErr               bool
		errorMessage          string
		mockCreateBehavior    func(mockFHIRClient *mock.MockClient)
	}{
		{
			name: "valid update - creator - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			existingPatientBundle: &existingPatientBundle,
			auditBundle:           &creationAuditBundle,
			wantErr:               false,
		},
		{
			name: "resource not found - creates new resource - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			existingPatientBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:               false,
		},
		{
			name: "invalid update - not authenticated - fails",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			wantErr:      true,
			errorMessage: "not authenticated",
		},
		{
			name: "invalid update - not creator - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			existingPatientBundle: &existingPatientBundle,
			auditBundle:           &creationAuditBundle,
			wantErr:               true,
			errorMessage:          "Participant does not have access to Patient",
		},
		{
			name: "invalid update - error searching existing resource - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			errorFromSearch: errors.New("failed to search for Patient"),
			wantErr:         true,
			errorMessage:    "failed to search for Patient",
		},
		{
			name: "invalid update - error querying audit events - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			existingPatientBundle: &existingPatientBundle,
			errorFromAuditQuery:   errors.New("failed to find creation AuditEvent"),
			wantErr:               true,
			errorMessage:          "Participant does not have access to Patient",
		},
		{
			name: "invalid update - no creation audit event - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
			},
			existingPatientBundle: &existingPatientBundle,
			auditBundle:           &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:               true,
			errorMessage:          "Participant does not have access to Patient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := coolfhir.Transaction()

			mockFHIRClient := mock.NewMockClient(ctrl)

			fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
			service := &Service{
				profile:    profile.Test(),
				fhirClient: mockFHIRClient,
				fhirURL:    fhirBaseUrl,
			}

			if tt.existingPatientBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(tt.ctx, "Patient", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingPatientBundle

					if len(tt.existingPatientBundle.Entry) > 0 {
						mockFHIRClient.EXPECT().SearchWithContext(tt.ctx, "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
							if tt.errorFromAuditQuery != nil {
								return tt.errorFromAuditQuery
							}
							*result = *tt.auditBundle
							return nil
						})
					}

					return nil
				})
			}

			if tt.mockCreateBehavior != nil {
				tt.mockCreateBehavior(mockFHIRClient)
			}

			result, err := service.handleUpdatePatient(tt.ctx, tt.request, tx)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorMessage != "" {
					require.Contains(t, err.Error(), tt.errorMessage)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
		})
	}
}
