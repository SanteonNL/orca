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

	questionnaireWithID := deep.Copy(defaultQuestionnaire)
	questionnaireWithID.Id = to.Ptr("existing-questionnaire-id")
	questionnaireWithIDJSON, _ := json.Marshal(questionnaireWithID)

	returnedBundleForUpdate := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Questionnaire/existing-questionnaire-id"),
					Status:   "200 OK",
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

	tests := []struct {
		name                       string
		questionnaireToCreate      fhir.Questionnaire
		createdQuestionnaireBundle *fhir.Bundle
		returnedBundle             *fhir.Bundle
		errorFromCreate            error
		expectError                error
		expectedMethod             string
		expectedURL                string
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
			expectedMethod: "POST",
			expectedURL:    "Questionnaire",
		},
		{
			name:                  "questionnaire with existing ID - update",
			questionnaireToCreate: questionnaireWithID,
			createdQuestionnaireBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: questionnaireWithIDJSON,
					},
				},
			},
			returnedBundle: returnedBundleForUpdate,
			expectedMethod: "PUT",
			expectedURL:    "Questionnaire/existing-questionnaire-id",
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

			// For resources with ID, expect a read from the specific ID path
			expectedLocation := "Questionnaire/1"
			if questionnaireToCreate.Id != nil {
				expectedLocation = "Questionnaire/" + *questionnaireToCreate.Id
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), expectedLocation, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdQuestionnaireBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			} else {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Questionnaire/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdQuestionnaireBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			}

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

				// Verify the request method and URL for the questionnaire entry
				if tt.expectedMethod != "" {
					questionnaireEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Questionnaire"))
					require.Equal(t, tt.expectedMethod, questionnaireEntry.Request.Method.String())
					require.Equal(t, tt.expectedURL, questionnaireEntry.Request.Url)
				}
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, *returnedBundle.Entry[0].Response.Location, *response.Response.Location)
			require.Equal(t, returnedBundle.Entry[0].Response.Status, response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
