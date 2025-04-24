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

	questionnaireResponseWithID := deep.Copy(defaultQuestionnaireResponse)
	questionnaireResponseWithID.Id = to.Ptr("existing-questionnaireresponse-id")
	questionnaireResponseWithIDJSON, _ := json.Marshal(questionnaireResponseWithID)

	returnedBundleForUpdate := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("QuestionnaireResponse/existing-questionnaireresponse-id"),
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
		name                               string
		questionnaireResponseToCreate      fhir.QuestionnaireResponse
		createdQuestionnaireResponseBundle *fhir.Bundle
		returnedBundle                     *fhir.Bundle
		errorFromCreate                    error
		expectError                        error
		principal                          *auth.Principal
		expectedMethod                     string
		expectedURL                        string
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
			expectedMethod: "POST",
			expectedURL:    "QuestionnaireResponse",
		},
		{
			name:                          "non-local requester - fails",
			questionnaireResponseToCreate: defaultQuestionnaireResponse,
			principal:                     auth.TestPrincipal2,
			expectError:                   errors.New("Participant is not allowed to create QuestionnaireResponse"),
		},
		{
			name:                          "questionnaireResponse with existing ID - update",
			questionnaireResponseToCreate: questionnaireResponseWithID,
			createdQuestionnaireResponseBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: questionnaireResponseWithIDJSON,
					},
				},
			},
			returnedBundle: returnedBundleForUpdate,
			expectedMethod: "PUT",
			expectedURL:    "QuestionnaireResponse/existing-questionnaireresponse-id",
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
				Principal: auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			tx := coolfhir.Transaction()

			mockFHIRClient := mock.NewMockClient(ctrl)
			fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
			handler := &FHIRCreateOperationHandler[fhir.QuestionnaireResponse]{
				profile:     profile.Test(),
				fhirClient:  mockFHIRClient,
				fhirURL:     fhirBaseUrl,
				authzPolicy: CreateQuestionnaireResponseAuthzPolicy(profile.Test()),
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)
			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.expectedMethod == "PUT" {
				fhirRequest.HttpMethod = "PUT"
				fhirRequest.Upsert = true
			}
			result, err := handler.Handle(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			// For resources with ID, expect a read from the specific ID path
			expectedLocation := "QuestionnaireResponse/1"
			if questionnaireResponseToCreate.Id != nil {
				expectedLocation = "QuestionnaireResponse/" + *questionnaireResponseToCreate.Id
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), expectedLocation, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdQuestionnaireResponseBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			} else {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "QuestionnaireResponse/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdQuestionnaireResponseBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			}

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

				// Verify the request method and URL for the questionnaireResponse entry
				if tt.expectedMethod != "" {
					questionnaireResponseEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("QuestionnaireResponse"))
					require.Equal(t, tt.expectedMethod, questionnaireResponseEntry.Request.Method.String())
					require.Equal(t, tt.expectedURL, questionnaireResponseEntry.Request.Url)
				}
			}

			// Process result
			require.NotNil(t, result)
			responses, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Len(t, responses, 1)
			require.Equal(t, *returnedBundle.Entry[0].Response.Location, *responses[0].Response.Location)
			require.Equal(t, returnedBundle.Entry[0].Response.Status, responses[0].Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
