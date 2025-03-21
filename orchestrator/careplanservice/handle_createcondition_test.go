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

func Test_handleCreateCondition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Condition/1"),
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

	defaultCondition := fhir.Condition{
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

	conditionWithID := deep.Copy(defaultCondition)
	conditionWithID.Id = to.Ptr("existing-condition-id")
	conditionWithIDJSON, _ := json.Marshal(conditionWithID)

	returnedBundleForUpdate := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Condition/existing-condition-id"),
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
		name                   string
		conditionToCreate      fhir.Condition
		createdCondition       fhir.Condition
		createdConditionBundle *fhir.Bundle
		returnedBundle         *fhir.Bundle
		expectError            error
		principal              *auth.Principal
		expectedMethod         string
		expectedURL            string
	}{
		{
			name: "happy flow",
			createdConditionBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: []byte(`{"clinicalStatus":{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/condition-clinical","code":"active","display":"Active"}]},"subject":{"identifier":{"system":"http://fhir.nl/fhir/NamingSystem/bsn","value":"1333333337"}},"code":{"coding":[{"system":"http://snomed.info/sct","code":"386661006","display":"Fever"}]}}`),
					},
				},
			},
		},
		{
			name:        "error: requester is not a local organization",
			principal:   auth.TestPrincipal2,
			expectError: errors.New("Only the local care organization can create a Condition"),
		},
		{
			name:              "condition with existing ID - update",
			conditionToCreate: conditionWithID,
			createdConditionBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: conditionWithIDJSON,
					},
				},
			},
			returnedBundle: returnedBundleForUpdate,
			expectedMethod: "PUT",
			expectedURL:    "Condition/existing-condition-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Condition
			var conditionToCreate = deep.Copy(defaultCondition)
			if !deep.Equal(tt.conditionToCreate, fhir.Condition{}) {
				conditionToCreate = tt.conditionToCreate
			}

			conditionBytes, _ := json.Marshal(conditionToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Condition",
				ResourceData: conditionBytes,
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
			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.expectedMethod == "PUT" {
				fhirRequest.HttpMethod = "PUT"
				fhirRequest.Upsert = true
			}
			result, err := service.handleCreateCondition(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			// For condition with ID, expect a different location path
			expectedLocation := "Condition/1"
			if conditionToCreate.Id != nil {
				expectedLocation = "Condition/" + *conditionToCreate.Id
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), expectedLocation, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdConditionBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			} else {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Condition/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdConditionBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			}

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify Condition", func(t *testing.T) {
					conditionEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Condition"))
					var condition fhir.Condition
					_ = json.Unmarshal(conditionEntry.Resource, &condition)
					require.Equal(t, conditionToCreate, condition)
				})

				// Verify the request method and URL for the condition entry
				if tt.expectedMethod != "" {
					conditionEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Condition"))
					require.Equal(t, tt.expectedMethod, conditionEntry.Request.Method.String())
					require.Equal(t, tt.expectedURL, conditionEntry.Request.Url)
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
