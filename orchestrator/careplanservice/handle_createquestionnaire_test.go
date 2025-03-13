package careplanservice

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func Test_handleCreateQuestionnaire(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Questionnaire/1"),
					Status:   "201 Created",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/2"),
					Status:   "201 Created",
				},
			},
		},
	}

	defaultQuestionnaire := fhir.Questionnaire{
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
	defaultQuestionnaireJSON, _ := json.Marshal(defaultQuestionnaire)

	tests := []struct {
		name                       string
		questionnaireToCreate      fhir.Questionnaire
		createdQuestionnaireBundle *fhir.Bundle
		returnedBundle             *fhir.Bundle
		errorFromCreate            error
		expectError                error
	}{
		{
			name:                  "happy flow - success",
			questionnaireToCreate: defaultQuestionnaire,
			createdQuestionnaireBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultQuestionnaireJSON,
					},
				},
			},
			returnedBundle: defaultReturnedBundle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Questionnaire
			var questionnaireToCreate = deep.Copy(defaultQuestionnaire)
			if !deep.Equal(tt.questionnaireToCreate, fhir.Questionnaire{}) {
				questionnaireToCreate = tt.questionnaireToCreate
			}

			questionnaireBytes, _ := json.Marshal(questionnaireToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Questionnaire",
				ResourceData: questionnaireBytes,
				HttpMethod:   "POST",
				HttpHeaders: map[string][]string{
					"If-None-Exist": {"ifnoneexist"},
				},
				Principal: auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			tx := coolfhir.Transaction()

			mockFHIRClient := mock.NewMockClient(ctrl)
			fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
			service := &Service{
				profile:    profile.Test(),
				fhirClient: mockFHIRClient,
				fhirURL:    fhirBaseUrl,
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

			result, err := service.handleCreateQuestionnaire(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Questionnaire/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = tt.createdQuestionnaireBundle.Entry[0].Resource
				return nil
			}).AnyTimes()

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify Questionnaire", func(t *testing.T) {
					questionnaireEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Questionnaire"))
					var questionnaire fhir.Questionnaire
					_ = json.Unmarshal(questionnaireEntry.Resource, &questionnaire)
					require.Equal(t, questionnaireToCreate, questionnaire)
				})
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, "Questionnaire/1", *response.Response.Location)
			require.Equal(t, "201 Created", response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
