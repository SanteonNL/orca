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

	tests := []struct {
		name                 string
		patientToCreate      fhir.Patient
		createdPatientBundle *fhir.Bundle
		returnedBundle       *fhir.Bundle
		errorFromCreate      error
		expectError          error
		ctx                  context.Context
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
		},
		{
			name:            "unauthorised - fails",
			patientToCreate: defaultPatient,
			ctx:             context.Background(),
			expectError:     errors.New("not authenticated"),
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

			result, err := service.handleCreatePatient(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "Patient/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = tt.createdPatientBundle.Entry[0].Resource
				return nil
			}).AnyTimes()

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
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, "Patient/1", *response.Response.Location)
			require.Equal(t, "201 Created", response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
