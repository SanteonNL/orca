package careplanservice

import (
	"context"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestService_handleGetCarePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	tests := []struct {
		ctx                 context.Context
		name                string
		id                  string
		returnedCarePlan    *fhir.CarePlan
		returnedCareTeam    *fhir.CareTeam
		expectErrorFromRead bool
		expectError         bool
	}{
		{
			ctx:                 context.Background(),
			name:                "CarePlan does not exist",
			id:                  "1",
			returnedCarePlan:    nil,
			returnedCareTeam:    nil,
			expectErrorFromRead: false,
			expectError:         true,
		},
		{
			ctx:  context.Background(),
			name: "No CareTeams returned",
			id:   "1",
			returnedCarePlan: &fhir.CarePlan{
				Id: to.Ptr("1"),
			},
			returnedCareTeam:    nil,
			expectErrorFromRead: false,
			expectError:         true,
		},
		// TODO: Positive test cases. These are complex to mock with the side effects of fhir.QueryParam, refactor unit tests to http tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				result = tt.returnedCarePlan
				if tt.expectErrorFromRead {
					return errors.New("error")
				}

				return nil
			})
			got, err := service.handleGetCarePlan(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedCarePlan, got)
			}
		})
	}
}

func TestService_handleSearchCarePlan(t *testing.T) {
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

	tests := []struct {
		ctx            context.Context
		name           string
		queryParams    url.Values
		returnedBundle *fhir.Bundle
		// used to validate result filtering, if needed
		expectedBundle *fhir.Bundle
		errorFromRead  error
		expectError    bool
	}{
		{
			ctx:         context.Background(),
			name:        "Empty bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         context.Background(),
			name:        "fhirclient error",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:         context.Background(),
			name:        "CarePlan, CareTeam returned, no auth",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
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
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:        "CarePlan, CareTeam returned, incorrect principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
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
			name:        "CarePlan, CareTeam returned, correct principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
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
						Resource: carePlan1,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "Multiple CarePlans, CareTeams returned, correct principal, results filtered",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1,
					},
					{
						Resource: carePlan2,
					},
					{
						Resource: careTeam1,
					},
					{
						Resource: careTeam2,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
				return tt.errorFromRead
			})

			got, err := service.handleSearchCarePlan(tt.ctx, tt.queryParams, &fhirclient.Headers{})
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
