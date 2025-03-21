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
		existingQuestionnaireBundle *fhir.Bundle
		errorFromSearch             error
		errorFromAuditQuery         error
		wantErr                     bool
		errorMessage                string
		mockCreateBehavior          func(mockFHIRClient *mock.MockClient)
		principal                   *auth.Principal
	}{
		{
			name:                        "valid update - success",
			principal:                   auth.TestPrincipal1,
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			wantErr:                     false,
		},
		{
			name:                        "resource not found - creates new resource - success",
			principal:                   auth.TestPrincipal1,
			existingQuestionnaireBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:                     false,
		},
		{
			name:                        "invalid update - error searching existing resource - fails",
			principal:                   auth.TestPrincipal1,
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
			errorFromSearch:             errors.New("failed to search for Questionnaire"),
			wantErr:                     true,
			errorMessage:                "failed to search for Questionnaire",
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
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)
			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
				ctx = auth.WithPrincipal(ctx, *tt.principal)
			}

			if tt.existingQuestionnaireBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingQuestionnaireBundle
					return nil
				})
			}

			if tt.mockCreateBehavior != nil {
				tt.mockCreateBehavior(mockFHIRClient)
			}

			result, err := service.handleUpdateQuestionnaire(ctx, fhirRequest, tx)

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
