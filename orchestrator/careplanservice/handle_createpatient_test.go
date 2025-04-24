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

func Test_handleCreatePatient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Patient/1"),
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

	defaultPatient := fhir.Patient{
		Name: []fhir.HumanName{
			{
				Given:  []string{"Jan"},
				Family: to.Ptr("Smit"),
			},
		},
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	defaultPatientJSON, _ := json.Marshal(defaultPatient)

	patientWithID := deep.Copy(defaultPatient)
	patientWithID.Id = to.Ptr("existing-patient-id")
	patientWithIDJSON, _ := json.Marshal(patientWithID)

	returnedBundleForUpdate := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("Patient/existing-patient-id"),
					Status:   "200 OK",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/3"),
					Status:   "201 Created",
				},
			},
		},
	}

	tests := []struct {
		name                 string
		patientToCreate      fhir.Patient
		createdPatientBundle *fhir.Bundle
		returnedBundle       *fhir.Bundle
		errorFromCreate      error
		expectError          error
		principal            *auth.Principal
		expectedMethod       string
		expectedURL          string
	}{
		{
			name:            "happy flow - success",
			patientToCreate: defaultPatient,
			createdPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultPatientJSON,
					},
				},
			},
			returnedBundle: defaultReturnedBundle,
			expectedMethod: "POST",
			expectedURL:    "Patient",
		},
		{
			name:            "non-local requester - fails",
			patientToCreate: defaultPatient,
			principal:       auth.TestPrincipal2,
			expectError:     errors.New("Participant is not allowed to create Patient"),
		},
		{
			name:            "patient with existing ID - update",
			patientToCreate: patientWithID,
			createdPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patientWithIDJSON,
					},
				},
			},
			returnedBundle: returnedBundleForUpdate,
			expectedMethod: "PUT",
			expectedURL:    "Patient/existing-patient-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a Patient
			var patientToCreate = deep.Copy(defaultPatient)
			if !deep.Equal(tt.patientToCreate, fhir.Patient{}) {
				patientToCreate = tt.patientToCreate
			}

			patientBytes, _ := json.Marshal(patientToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "Patient",
				ResourceData: patientBytes,
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
			handler := &FHIRCreateOperationHandler[fhir.Patient]{
				profile:     profile.Test(),
				fhirClient:  mockFHIRClient,
				fhirURL:     fhirBaseUrl,
				authzPolicy: CreatePatientAuthzPolicy(profile.Test()),
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

			// For patient with ID, expect a different location path
			expectedLocation := "Patient/1"
			if patientToCreate.Id != nil {
				expectedLocation = "Patient/" + *patientToCreate.Id
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), expectedLocation, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdPatientBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			} else {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Patient/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdPatientBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			}

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify Patient", func(t *testing.T) {
					patientEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Patient"))
					var patient fhir.Patient
					_ = json.Unmarshal(patientEntry.Resource, &patient)
					require.Equal(t, patientToCreate, patient)
				})

				// Verify the request method and URL for the patient entry
				if tt.expectedMethod != "" {
					patientEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("Patient"))
					require.Equal(t, tt.expectedMethod, patientEntry.Request.Method.String())
					require.Equal(t, tt.expectedURL, patientEntry.Request.Url)
				}
			}

			// Process result
			require.NotNil(t, result)
			responses, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, *returnedBundle.Entry[0].Response.Location, *responses[0].Response.Location)
			require.Equal(t, returnedBundle.Entry[0].Response.Status, responses[0].Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
