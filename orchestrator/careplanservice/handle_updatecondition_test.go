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

func Test_handleUpdateCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultCondition := fhir.Condition{
		Id: to.Ptr("1"),
		ClinicalStatus: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://terminology.hl7.org/CodeSystem/condition-clinical"),
					Code:    to.Ptr("active"),
					Display: to.Ptr("Active"),
				},
			},
		},
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
					Code:    to.Ptr("386661006"),
					Display: to.Ptr("Fever"),
				},
			},
		},
	}

	updateConditionData, _ := json.Marshal(defaultCondition)

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
					Reference: to.Ptr("Condition/1"),
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

	existingConditionBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultCondition)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                    string
		principal               *auth.Principal
		existingConditionBundle *fhir.Bundle
		errorFromSearch         error
		errorFromAuditQuery     error
		auditBundle             *fhir.Bundle
		wantErr                 bool
		errorMessage            string
		mockCreateBehavior      func(mockFHIRClient *mock.MockClient)
	}{
		{
			name:                    "valid update - creator - success",
			principal:               auth.TestPrincipal1,
			existingConditionBundle: &existingConditionBundle,
			auditBundle:             &creationAuditBundle,
			wantErr:                 false,
		},
		{
			name:                    "resource not found - creates new resource - success",
			principal:               auth.TestPrincipal1,
			existingConditionBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                 false,
		},
		{
			name:                    "invalid update - not creator - fails",
			principal:               auth.TestPrincipal2,
			existingConditionBundle: &existingConditionBundle,
			auditBundle:             &creationAuditBundle,
			wantErr:                 true,
			errorMessage:            "Participant does not have access to Condition",
		},
		{
			name:            "invalid update - error searching existing resource - fails",
			principal:       auth.TestPrincipal1,
			errorFromSearch: errors.New("failed to search for Condition"),
			wantErr:         true,
			errorMessage:    "failed to search for Condition",
		},
		{
			name:                    "invalid update - error querying audit events - fails",
			principal:               auth.TestPrincipal1,
			existingConditionBundle: &existingConditionBundle,
			errorFromAuditQuery:     errors.New("failed to find creation AuditEvent"),
			wantErr:                 true,
			errorMessage:            "Participant does not have access to Condition",
		},
		{
			name:                    "invalid update - no creation audit event - fails",
			principal:               auth.TestPrincipal1,
			existingConditionBundle: &existingConditionBundle,
			auditBundle:             &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                 true,
			errorMessage:            "Participant does not have access to Condition",
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
				ResourceData: updateConditionData,
				ResourcePath: "Condition/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			}

			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.existingConditionBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "Condition", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingConditionBundle

					if len(tt.existingConditionBundle.Entry) > 0 {
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

			result, err := service.handleUpdateCondition(ctx, fhirRequest, tx)

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
