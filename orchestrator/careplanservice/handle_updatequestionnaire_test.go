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

func Test_handleUpdateQuestionnaire(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultQuestionnaire := fhir.Questionnaire{
		Id:     to.Ptr("1"),
		Status: fhir.PublicationStatusDraft,
		Title:  to.Ptr("Test Questionnaire"),
		Item: []fhir.QuestionnaireItem{
			{
				LinkId: "1",
				Text:   to.Ptr("Question 1"),
				Type:   fhir.QuestionnaireItemTypeString,
			},
		},
	}

	updateQuestionnaireData, _ := json.Marshal(defaultQuestionnaire)

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
					Reference: to.Ptr("Questionnaire/1"),
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

	existingQuestionnaireBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultQuestionnaire)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                        string
		ctx                         context.Context
		request                     FHIRHandlerRequest
		existingQuestionnaireBundle *fhir.Bundle
		errorFromSearch             error
		errorFromAuditQuery         error
		auditBundle                 *fhir.Bundle
		wantErr                     bool
		errorMessage                string
		mockCreateBehavior          func(mockFHIRClient *mock.MockClient)
	}{
		{
			name: "valid update - creator - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			auditBundle:                 &creationAuditBundle,
			wantErr:                     false,
		},
		{
			name: "resource not found - creates new resource - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                     false,
		},
		{
			name: "invalid update - not authenticated - fails",
			ctx:  context.Background(),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			wantErr:      true,
			errorMessage: "not authenticated",
		},
		{
			name: "invalid update - not creator - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			auditBundle:                 &creationAuditBundle,
			wantErr:                     true,
			errorMessage:                "Only the creator can update this Questionnaire",
		},
		{
			name: "invalid update - error searching existing resource - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			errorFromSearch: errors.New("failed to search for Questionnaire"),
			wantErr:         true,
			errorMessage:    "failed to search for Questionnaire",
		},
		{
			name: "invalid update - error querying audit events - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			errorFromAuditQuery:         errors.New("failed to find creation AuditEvent"),
			wantErr:                     true,
			errorMessage:                "failed to find creation AuditEvent",
		},
		{
			name: "invalid update - no creation audit event - fails",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			auditBundle:                 &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                     true,
			errorMessage:                "No creation audit event found for Questionnaire",
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

			if tt.existingQuestionnaireBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().Search("Questionnaire", gomock.Any(), gomock.Any()).DoAndReturn(func(resourceType string, params url.Values, result interface{}, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*(result.(*fhir.Bundle)) = *tt.existingQuestionnaireBundle

					if len(tt.existingQuestionnaireBundle.Entry) > 0 {
						mockFHIRClient.EXPECT().Search("AuditEvent", gomock.Any(), gomock.Any()).DoAndReturn(func(resourceType string, params url.Values, result interface{}, option ...fhirclient.Option) error {
							if tt.errorFromAuditQuery != nil {
								return tt.errorFromAuditQuery
							}
							*(result.(*fhir.Bundle)) = *tt.auditBundle
							return nil
						})
					}

					return nil
				})
			}

			if tt.mockCreateBehavior != nil {
				tt.mockCreateBehavior(mockFHIRClient)
			}

			result, err := service.handleUpdateQuestionnaire(tt.ctx, tt.request, tx)

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
