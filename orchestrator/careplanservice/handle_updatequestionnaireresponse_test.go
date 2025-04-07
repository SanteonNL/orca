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

func Test_handleUpdateQuestionnaireResponse(t *testing.T) {
	t.Skip("Temporarily skipping")
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultQuestionnaireResponse := fhir.QuestionnaireResponse{
		Id:            to.Ptr("1"),
		Status:        fhir.QuestionnaireResponseStatusInProgress,
		Questionnaire: to.Ptr("Questionnaire/123"),
		Subject: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}

	updateQuestionnaireResponseData, _ := json.Marshal(defaultQuestionnaireResponse)

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
					Reference: to.Ptr("QuestionnaireResponse/1"),
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

	existingQuestionnaireResponseBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultQuestionnaireResponse)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                           string
		existingQuestionnaireResBundle *fhir.Bundle
		errorFromSearch                error
		errorFromAuditQuery            error
		auditBundle                    *fhir.Bundle
		mockCreateBehavior             func(mockFHIRClient *mock.MockClient)
		wantErr                        bool
		errorMessage                   string
		principal                      *auth.Principal
	}{
		{
			name:                           "valid update - creator - success",
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &creationAuditBundle,
			wantErr:                        false,
			principal:                      auth.TestPrincipal1,
		},
		{
			name:                           "invalid update - not creator - fails",
			principal:                      auth.TestPrincipal2,
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &creationAuditBundle,
			wantErr:                        true,
			errorMessage:                   "Participant does not have access to QuestionnaireResponse",
		},
		{
			name:            "invalid update - error searching existing resource - fails",
			principal:       auth.TestPrincipal1,
			errorFromSearch: errors.New("failed to read QuestionnaireResponse"),
			wantErr:         true,
			errorMessage:    "failed to read QuestionnaireResponse",
		},
		{
			name:                           "invalid update - error querying audit events - fails",
			principal:                      auth.TestPrincipal1,
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			errorFromAuditQuery:            errors.New("failed to find creation AuditEvent"),
			wantErr:                        true,
			errorMessage:                   "Participant does not have access to QuestionnaireResponse",
		},
		{
			name:                           "invalid update - no creation audit event - fails",
			principal:                      auth.TestPrincipal1,
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                        true,
			errorMessage:                   "Participant does not have access to QuestionnaireResponse",
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
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				},
			}

			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.existingQuestionnaireResBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "QuestionnaireResponse", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingQuestionnaireResBundle

					if len(tt.existingQuestionnaireResBundle.Entry) > 0 {
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

			if tt.mockCreateBehavior != nil {
				tt.mockCreateBehavior(mockFHIRClient)
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

			result, err := service.handleUpdateQuestionnaireResponse(ctx, fhirRequest, tx)

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
