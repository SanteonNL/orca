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

func TestService_handleGetTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	tests := []struct {
		ctx              context.Context
		name             string
		id               string
		returnedTask     *fhir.Task
		returnedCarePlan *fhir.CarePlan
		returnedCareTeam *fhir.CareTeam
		errorFromRead    error
		expectError      bool
	}{
		{
			ctx:              context.Background(),
			name:             "Task does not exist",
			id:               "1",
			returnedTask:     nil,
			returnedCarePlan: nil,
			returnedCareTeam: nil,
			errorFromRead:    errors.New("error"),
			expectError:      true,
		},
		// TODO: These are complex to mock with the side effects of fhir.QueryParam, refactor unit tests to http tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.returnedCarePlan != nil {
				mockFHIRClient.EXPECT().Read("CarePlan/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedCarePlan))
					return tt.errorFromRead
				})
			}
			mockFHIRClient.EXPECT().Read("Task/1", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if tt.returnedTask != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*tt.returnedTask))
				}
				return tt.errorFromRead
			})
			got, err := service.handleGetTask(tt.ctx, tt.id, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedTask, got)
			}
		})
	}
}

func TestService_handleSearchTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	rawCarePlan, _ := os.ReadFile("./testdata/careplan-1.json")
	rawCareTeam, _ := os.ReadFile("./testdata/careteam-2.json")

	tests := []struct {
		ctx            context.Context
		name           string
		queryParams    url.Values
		returnedBundle *fhir.Bundle
		errorFromRead  error
		expectError    bool
	}{
		{
			ctx:         context.Background(),
			name:        "No CareTeam in bundle",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: nil,
			expectError:   true,
		},
		{
			ctx:         context.Background(),
			name:        "CareTeam present in Bundle, no auth",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: rawCarePlan,
					},
					{
						Resource: rawCareTeam,
					},
				},
			},
			errorFromRead: nil,
			expectError:   true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:        "CareTeam present in Bundle, incorrect principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: rawCarePlan,
					},
					{
						Resource: rawCareTeam,
					},
				},
			},
			errorFromRead: nil,
			expectError:   true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:        "fhirClient error",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:         auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:        "CareTeam present in Bundle, correct principal",
			queryParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: rawCarePlan,
					},
					{
						Resource: rawCareTeam,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
		{
			ctx:  auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name: "CareTeam present in Bundle, correct principal, Request careTeam in headers",
			queryParams: url.Values{
				"_include": []string{"CarePlan:care-team"},
			},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: rawCarePlan,
					},
					{
						Resource: rawCareTeam,
					},
				},
			},
			errorFromRead: nil,
			expectError:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ string, resultResource interface{}, _ ...fhirclient.Option) error {
					// The FHIR client reads the resource from the FHIR server, to return it to the client.
					// In this test, we return the expected ServiceRequest.
					reflect.ValueOf(resultResource).Elem().Set(reflect.ValueOf(*tt.returnedBundle))
					return tt.errorFromRead
				})
			got, err := service.handleSearchCarePlan(tt.ctx, tt.queryParams, &fhirclient.Headers{})
			if tt.expectError == true {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.returnedBundle, got)
			}
		})
	}
}
