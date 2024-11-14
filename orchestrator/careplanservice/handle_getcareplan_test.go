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

func TestService_handleGetCarePlan(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")

	tests := []struct {
		ctx                    context.Context
		name                   string
		id                     string
		returnedCarePlanBundle *fhir.Bundle
		expectedCarePlan       *fhir.CarePlan
		errorFromRead          error
		expectError            bool
	}{
		{
			ctx:         context.Background(),
			name:        "No auth",
			id:          "1",
			expectError: true,
		},
		{
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:                   "CarePlan does not exist",
			id:                     "1",
			errorFromRead:          errors.New("error"),
			returnedCarePlanBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectError:            true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "No CareTeams returned",
			id:   "1",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1Raw,
					},
				},
			},
			expectError: true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name: "CarePlan, CareTeam returned, incorrect principal",
			id:   "1",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1Raw,
					},
					{
						Resource: careTeam2Raw,
					},
				},
			},
			expectError: true,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CarePlan, CareTeam returned, correct principal",
			id:   "1",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1Raw,
					},
					{
						Resource: careTeam2Raw,
					},
				},
			},
			expectError:      false,
			expectedCarePlan: &carePlan1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedCarePlanBundle != nil || tt.errorFromRead != nil {
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCarePlanBundle))
					return tt.errorFromRead
				})
			}
			got, err := service.handleGetCarePlan(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedCarePlan, got)
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
			name:        "CarePlan, CareTeam returned, correct principal, include careteam",
			queryParams: url.Values{"_include": []string{"CarePlan:care-team"}},
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
			name:        "CarePlan, CareTeam returned, correct principal, do not include careteam",
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
			queryParams: url.Values{"_include": []string{"CarePlan:care-team"}},
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
				mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
					return tt.errorFromRead
				})
			}

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
