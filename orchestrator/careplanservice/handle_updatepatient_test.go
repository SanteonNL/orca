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

func Test_handleUpdatePatient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultPatient := fhir.Patient{
		Id: to.Ptr("1"),
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

	updatePatientData, _ := json.Marshal(defaultPatient)

	existingPatientBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Resource: func() []byte {
					b, _ := json.Marshal(defaultPatient)
					return b
				}(),
			},
		},
	}

	tests := []struct {
		name                  string
		existingPatientBundle *fhir.Bundle
		errorFromSearch       error
		wantErr               bool
		errorMessage          string
		mockCreateBehavior    func(mockFHIRClient *mock.MockClient)
		principal             *auth.Principal
	}{
		{
			name:                  "valid update - creator - success",
			principal:             auth.TestPrincipal1,
			existingPatientBundle: &existingPatientBundle,
			wantErr:               false,
		},
		{
			name:                  "resource not found - creates new resource - success",
			principal:             auth.TestPrincipal1,
			existingPatientBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			wantErr:               false,
		},
		{
			name:            "invalid update - error searching existing resource - fails",
			principal:       auth.TestPrincipal1,
			errorFromSearch: errors.New("failed to search for Patient"),
			wantErr:         true,
			errorMessage:    "failed to search for Patient",
		},
		// TODO: Re-implement, test case is still valid but auth mechanism needs to change
		//{
		//	name:                  "invalid update - error querying audit events - fails",
		//	principal:             auth.TestPrincipal1,
		//	existingPatientBundle: &existingPatientBundle,
		//	errorFromAuditQuery:   errors.New("failed to find creation AuditEvent"),
		//	wantErr:               true,
		//	errorMessage:          "Participant does not have access to Patient",
		//},
		//{
		//	name:                  "invalid update - no creation audit event - fails",
		//	principal:             auth.TestPrincipal1,
		//	existingPatientBundle: &existingPatientBundle,
		//	wantErr:               true,
		//	errorMessage:          "Participant does not have access to Patient",
		//},
		//{
		//	name:                  "invalid update - not creator - fails",
		//	principal:             auth.TestPrincipal2,
		//	existingPatientBundle: &existingPatientBundle,
		//	wantErr:               true,
		//	errorMessage:          "Participant does not have access to Patient",
		//},
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

			if tt.existingPatientBundle != nil || tt.errorFromSearch != nil {
				mockFHIRClient.EXPECT().SearchWithContext(gomock.Any(), "Patient", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, resourceType string, params url.Values, result *fhir.Bundle, option ...fhirclient.Option) error {
					if tt.errorFromSearch != nil {
						return tt.errorFromSearch
					}
					*result = *tt.existingPatientBundle

					return nil
				})
			}

			if tt.mockCreateBehavior != nil {
				tt.mockCreateBehavior(mockFHIRClient)
			}

			fhirRequest := FHIRHandlerRequest{
				ResourceId:   "1",
				ResourceData: updatePatientData,
				ResourcePath: "Patient/1",
				Principal:    auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
			}

			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

			result, err := service.handleUpdatePatient(ctx, fhirRequest, tx)

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
