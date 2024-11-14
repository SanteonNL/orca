package careplanservice

import (
	"context"
	"encoding/json"
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

func TestService_handleGetCareTeam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := []struct {
		ctx              context.Context
		name             string
		id               string
		returnedCareTeam *fhir.CareTeam
		errorFromRead    error
		expectError      bool
	}{
		{
			ctx:         context.Background(),
			name:        "No auth",
			id:          "2",
			expectError: true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "CareTeam does not exist",
			id:               "2",
			returnedCareTeam: &fhir.CareTeam{},
			errorFromRead:    errors.New("error"),
			expectError:      true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "CareTeam exists, auth, incorrect principal",
			id:               "2",
			returnedCareTeam: &careTeam2,
			expectError:      true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:             "CareTeam exists, auth, correct principal",
			id:               "2",
			returnedCareTeam: &careTeam2,
			expectError:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedCareTeam != nil || tt.errorFromRead != nil {
				mockFHIRClient.EXPECT().Read("CareTeam/2", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCareTeam))
					return tt.errorFromRead
				})
			}
			got, err := service.handleGetCareTeam(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedCareTeam, got)
			}
		})
	}
}

func TestService_handleSearchCareTeam(t *testing.T) {
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
			name:        "CareTeam returned, auth",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careTeam2,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "CareTeam returned, auth, multiple CareTeams but only access to 1",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
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
						Resource: careTeam2,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "CareTeam and CarePlan returned, auth, multiple CareTeams but only access to 1, only CareTeams are filtered",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careTeam1,
					},
					{
						Resource: careTeam2,
					},
					{
						Resource: carePlan1,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careTeam2,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedBundle != nil || tt.errorFromRead != nil {
				mockFHIRClient.EXPECT().Read("CareTeam", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
					return tt.errorFromRead
				})
			}

			got, err := service.handleSearchCareTeam(tt.ctx, tt.queryParams, &fhirclient.Headers{})
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
