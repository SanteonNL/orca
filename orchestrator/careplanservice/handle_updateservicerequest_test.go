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

func Test_handleUpdateServiceRequest(t *testing.T) {
	t.Skip("Temporarily skipping")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultServiceRequest := fhir.ServiceRequest{
		Id:     to.Ptr("1"),
		Status: fhir.RequestStatusActive,
		Intent: fhir.RequestIntentOrder,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("123456"),
					Display: to.Ptr("Test Service"),
				},
			},
		},
	}

	updateServiceRequestData, _ := json.Marshal(defaultServiceRequest)

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
					Reference: to.Ptr("ServiceRequest/1"),
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

	existingServiceRequestBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultServiceRequest)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                         string
		principal                    *auth.Principal
		existingServiceRequestBundle *fhir.Bundle
		errorFromSearch              error
		errorFromAuditQuery          error
		auditBundle                  *fhir.Bundle
		wantErr                      bool
		errorMessage                 string
		mockCreateBehavior           func(mockFHIRClient *mock.MockClient)
	}{
		{
			name:                         "valid update - creator - success",
			principal:                    auth.TestPrincipal1,
			existingServiceRequestBundle: &existingServiceRequestBundle,
			auditBundle:                  &creationAuditBundle,
			wantErr:                      false,
		},
		{
			name:                         "resource not found - creates new resource - success",
			principal:                    auth.TestPrincipal1,
			existingServiceRequestBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                      false,
		},
		{
			name:                         "invalid update - not creator - fails",
			principal:                    auth.TestPrincipal2,
			existingServiceRequestBundle: &existingServiceRequestBundle,
			auditBundle:                  &creationAuditBundle,
			wantErr:                      true,
			errorMessage:                 "Participant does not have access to ServiceRequest",
		},
		{
			name:            "invalid update - error searching existing resource - fails",
			principal:       auth.TestPrincipal1,
			errorFromSearch: errors.New("failed to search for ServiceRequest"),
			wantErr:         true,
			errorMessage:    "failed to search for ServiceRequest",
		},
		{
			name:                         "invalid update - error querying audit events - fails",
			principal:                    auth.TestPrincipal1,
			existingServiceRequestBundle: &existingServiceRequestBundle,
			errorFromAuditQuery:          errors.New("failed to find creation AuditEvent"),
			wantErr:                      true,
			errorMessage:                 "Participant does not have access to ServiceRequest",
		},
		{
			name:                         "invalid update - no creation audit event - fails",
			principal:                    auth.TestPrincipal1,
			existingServiceRequestBundle: &existingServiceRequestBundle,
			auditBundle:                  &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                      true,
			errorMessage:                 "Participant does not have access to ServiceRequest",
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

			fhirRequest := FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateServiceRequestData,
				ResourcePath: "ServiceRequest/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			}

			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.existingServiceRequestBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "ServiceRequest", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingServiceRequestBundle

					if len(tt.existingServiceRequestBundle.Entry) > 0 {
						mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "AuditEvent", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
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

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

			result, err := service.handleUpdateServiceRequest(ctx, fhirRequest, tx)

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
