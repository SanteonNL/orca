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
		ctx                         context.Context
		request                     FHIRHandlerRequest
		existingQuestionnaireBundle *fhir.Bundle
		errorFromSearch             error
		errorFromAuditQuery         error
		wantErr                     bool
		errorMessage                string
		mockCreateBehavior          func(mockFHIRClient *mock.MockClient)
	}{
		{
			name: "valid update - success",
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			request: FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updateQuestionnaireData,
				ResourcePath: "Questionnaire/1",
			},
			existingQuestionnaireBundle: &existingQuestionnaireBundle,
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
				mockFHIRClient.EXPECT().SearchWithContext(tt.ctx, "Questionnaire", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result interface{}, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*(result.(*fhir.Bundle)) = *tt.existingQuestionnaireBundle
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
