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
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func Test_handleCreateQuestionnaireResponse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("QuestionnaireResponse/1"),
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

	defaultQuestionnaireResponse := fhir.QuestionnaireResponse{
		Status:        fhir.QuestionnaireResponseStatusInProgress,
		Questionnaire: to.Ptr("Questionnaire/123"),
		Subject: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	defaultQuestionnaireResponseJSON, _ := json.Marshal(defaultQuestionnaireResponse)

	tests := []struct {
		name                               string
		questionnaireResponseToCreate      fhir.QuestionnaireResponse
		createdQuestionnaireResponseBundle *fhir.Bundle
		returnedBundle                     *fhir.Bundle
		errorFromCreate                    error
		expectError                        error
		ctx                                context.Context
	}{
		{
			name:                          "happy flow - success",
			questionnaireResponseToCreate: defaultQuestionnaireResponse,
			createdQuestionnaireResponseBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultQuestionnaireResponseJSON,
					},
				},
			},
			returnedBundle: defaultReturnedBundle,
		},
		{
			name:                          "unauthorised - fails",
			questionnaireResponseToCreate: defaultQuestionnaireResponse,
			ctx:                           context.Background(),
			expectError:                   errors.New("not authenticated"),
		},
		{
			name:                          "non-local requester - fails",
			questionnaireResponseToCreate: defaultQuestionnaireResponse,
			ctx:                           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			expectError:                   errors.New("Only the local care organization can create a QuestionnaireResponse"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a QuestionnaireResponse
			var questionnaireResponseToCreate = deep.Copy(defaultQuestionnaireResponse)
			if !deep.Equal(tt.questionnaireResponseToCreate, fhir.QuestionnaireResponse{}) {
				questionnaireResponseToCreate = tt.questionnaireResponseToCreate
			}

			questionnaireResponseBytes, _ := json.Marshal(questionnaireResponseToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "QuestionnaireResponse",
				ResourceData: questionnaireResponseBytes,
				HttpMethod:   "POST",
				HttpHeaders: map[string][]string{
					"If-None-Exist": {"ifnoneexist"},
				},
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
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			result, err := service.handleCreateQuestionnaireResponse(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = tt.createdQuestionnaireResponseBundle.Entry[0].Resource
				return nil
			}).AnyTimes()

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify QuestionnaireResponse", func(t *testing.T) {
					questionnaireResponseEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("QuestionnaireResponse"))
					var questionnaireResponse fhir.QuestionnaireResponse
					_ = json.Unmarshal(questionnaireResponseEntry.Resource, &questionnaireResponse)
					require.Equal(t, questionnaireResponseToCreate, questionnaireResponse)
				})
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, "QuestionnaireResponse/1", *response.Response.Location)
			require.Equal(t, "201 Created", response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
