package careplanservice

import (
	"context"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"os"
	"reflect"
	"testing"
)

// TODO: Refactor into shared Search tests when other Search CRUD methods are implemented
func TestService_handleSearchPatient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	careTeam1, _ := os.ReadFile("./testdata/careteam-1.json")
	careTeam2, _ := os.ReadFile("./testdata/careteam-2.json")
	carePlan1, _ := os.ReadFile("./testdata/careplan-1.json")
	carePlan2, _ := os.ReadFile("./testdata/careplan-2.json")
	patient1, _ := os.ReadFile("./testdata/patient-1.json")
	patient2, _ := os.ReadFile("./testdata/patient-2.json")

	tests := []struct {
		ctx                    context.Context
		name                   string
		queryParams            url.Values
		returnedBundle         *fhir.Bundle
		errorFromRead          error
		returnedCarePlanBundle *fhir.Bundle
		errorFromCarePlanRead  error
		// used to validate result filtering, if needed
		expectedBundle *fhir.Bundle
		expectError    bool
	}{
		{
			ctx:         context.Background(),
			name:        "No auth",
			queryParams: url.Values{},
			expectError: true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Empty bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "fhirclient error",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Patient returned, error from CarePlan read",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			errorFromCarePlanRead: errors.New("error"),
			errorFromRead:         nil,
			expectError:           true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Patient returned, no careplan or careteam returned",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:        "Patient returned, careplan and careteam returned, incorrect principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1,
					},
					{
						Resource: careTeam2,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Patient returned, careplan and careteam returned, correct principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1,
					},
					{
						Resource: careTeam2,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Multiple resources returned, correctly filtered",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
					{
						Resource: patient2,
					},
				},
			},
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1,
					},
					{
						Resource: careTeam2,
					},
					{
						Resource: carePlan2,
					},
					{
						Resource: careTeam1,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedCarePlanBundle != nil || tt.errorFromCarePlanRead != nil {
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					if tt.returnedCarePlanBundle != nil {
						reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCarePlanBundle))
					}
					return tt.errorFromCarePlanRead
				})
			}
			if tt.returnedBundle != nil || tt.errorFromRead != nil {
				mockFHIRClient.EXPECT().Read("Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					if tt.returnedBundle != nil {
						reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
					}
					return tt.errorFromRead
				})
			}

			got, err := service.handleSearchPatient(tt.ctx, tt.queryParams, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.expectedBundle != nil {
					require.Equal(t, tt.expectedBundle, got)
					return
				}
				require.Equal(t, tt.returnedBundle, got)
			}
		})
	}
}
