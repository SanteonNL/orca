package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/url"
	"os"
	"testing"
)

func TestService_handleGetCarePlan(t *testing.T) {
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	tests := []TestHandleGetStruct[fhir.CarePlan]{
		{
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:                   "error: CarePlan does not exist",
			id:                     "1",
			resourceType:           "CarePlan",
			returnedCarePlanBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectedError:          errors.New("entry not found in FHIR Bundle"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: No CareTeams returned",
			id:           "1",
			resourceType: "CarePlan",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: carePlan1Raw,
					},
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "CareTeam not found in bundle",
				StatusCode: http.StatusNotFound,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "error: CarePlan, CareTeam returned, incorrect principal",
			id:           "1",
			resourceType: "CarePlan",
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
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant is not part of CareTeam",
				StatusCode: http.StatusForbidden,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ok: CarePlan, CareTeam returned, correct principal",
			id:           "1",
			resourceType: "CarePlan",
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
			expectedError:    nil,
			returnedResource: &carePlan1,
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	for _, tt := range tests {
		tt.mockClient = mockFHIRClient
		testHelperHandleGetResource[fhir.CarePlan](t, tt, service.handleGetCarePlan)
	}
}

func TestService_handleSearchCarePlan(t *testing.T) {
	careTeam1, _ := os.ReadFile("./testdata/careteam-1.json")
	careTeam2, _ := os.ReadFile("./testdata/careteam-2.json")
	carePlan1, _ := os.ReadFile("./testdata/careplan-1.json")
	carePlan2, _ := os.ReadFile("./testdata/careplan-2.json")

	tests := []TestHandleSearchStruct[fhir.CarePlan]{
		{
			ctx:           context.Background(),
			resourceType:  "CarePlan",
			name:          "No auth",
			searchParams:  url.Values{},
			expectedError: errors.New("not authenticated"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "CarePlan",
			name:         "Empty bundle",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "CarePlan",
			name:         "fhirclient error",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromSearch: errors.New("error"),
			expectedError:   errors.New("error"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			resourceType: "CarePlan",
			name:         "CarePlan, CareTeam returned, incorrect principal",
			searchParams: url.Values{},
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
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "CarePlan",
			name:         "CarePlan, CareTeam returned, correct principal, include careteam",
			searchParams: url.Values{"_include": []string{"CarePlan:care-team"}},
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
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "CarePlan",
			name:         "CarePlan, CareTeam returned, correct principal, do not include careteam",
			searchParams: url.Values{},
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
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "CarePlan",
			name:         "Multiple CarePlans, CareTeams returned, correct principal, results filtered",
			searchParams: url.Values{"_include": []string{"CarePlan:care-team"}},
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
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockClient = mockFHIRClient
			testHelperHandleSearchResource[fhir.CarePlan](t, tt, service.handleSearchCarePlan)
		})
	}
}
