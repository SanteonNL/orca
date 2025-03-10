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
		ctx                            context.Context
		request                        FHIRHandlerRequest
		existingQuestionnaireResBundle *fhir.Bundle
		errorFromSearch                error
		errorFromAuditQuery            error
		auditBundle                    *fhir.Bundle
		mockCreateBehavior             func(mockFHIRClient *mock.MockClient)
		wantErr                        bool
		errorMessage                   string
	}{
		{
			name: "valid update - creator - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &creationAuditBundle,
			wantErr:                        false,
		},
		{
			name: "invalid update - not authenticated - fails",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			wantErr:      true,
			errorMessage: "not authenticated",
		},
		{
			name: "invalid update - not creator - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &creationAuditBundle,
			wantErr:                        true,
			errorMessage:                   "Only the creator can update this QuestionnaireResponse",
		},
		{
			name: "invalid update - error searching existing resource - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			errorFromSearch: errors.New("failed to read QuestionnaireResponse"),
			wantErr:         true,
			errorMessage:    "failed to read QuestionnaireResponse",
		},
		{
			name: "invalid update - error querying audit events - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			errorFromAuditQuery:            errors.New("failed to find creation AuditEvent"),
			wantErr:                        true,
			errorMessage:                   "failed to find creation AuditEvent",
		},
		{
			name: "invalid update - no creation audit event - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireResponseData,
				ResourcePath: "QuestionnaireResponse/1",
			},
			existingQuestionnaireResBundle: &existingQuestionnaireResponseBundle,
			auditBundle:                    &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                        true,
			errorMessage:                   "No creation audit event found for QuestionnaireResponse",
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

			if tt.existingQuestionnaireResBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(tt.ctx, "QuestionnaireResponse", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingQuestionnaireResBundle

					if len(tt.existingQuestionnaireResBundle.Entry) > 0 {
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

			result, err := service.handleUpdateQuestionnaireResponse(tt.ctx, tt.request, tx)

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
