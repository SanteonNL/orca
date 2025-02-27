package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestService_handleGetPatient(t *testing.T) {
	carePlan1Raw := mustReadFile("./testdata/careplan1-careteam2.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw := mustReadFile("./testdata/careplan2-careteam1.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	patient1Raw := mustReadFile("./testdata/patient-1.json")
	var patient1 fhir.Patient
	_ = json.Unmarshal(patient1Raw, &patient1)

	tests := []TestHandleGetStruct[fhir.Patient]{
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Patient does not exist",
			id:           "1",
			resourceType: "Patient",
			errorFromRead: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			expectedError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Patient exists, auth, error fetching CarePlan",
			id:           "1",
			resourceType: "Patient",
			errorFromCarePlanRead: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			expectedError: fhirclient.OperationOutcomeError{
				HttpStatusCode: http.StatusNotFound,
			},
			returnedResource: &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "error: Patient exists, auth, No CarePlans returned",
			id:           "1",
			resourceType: "Patient",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			returnedResource: &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "error: Patient exists, auth, CarePlan and CareTeam returned, not a participant",
			id:           "1",
			resourceType: "Patient",
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
				Message:    "Participant does not have access to Patient",
				StatusCode: http.StatusForbidden,
			},
			returnedResource: &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ok: Patient exists, auth, CarePlan and CareTeam returned, correct principal",
			id:           "1",
			resourceType: "Patient",
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
			returnedResource: &patient1,
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
		testHelperHandleGetResource[fhir.Patient](t, tt, service.handleGetPatient)
	}
}

func TestService_handleSearchPatient(t *testing.T) {
	careplan1Careteam2 := mustReadFile("./testdata/careplan1-careteam2.json")
	careplan2Careteam1 := mustReadFile("./testdata/careplan2-careteam1.json")
	patient1 := mustReadFile("./testdata/patient-1.json")
	patient2 := mustReadFile("./testdata/patient-2.json")

	tests := []TestHandleSearchStruct[fhir.Patient]{
		{
			ctx:           context.Background(),
			resourceType:  "Patient",
			name:          "No auth",
			searchParams:  url.Values{},
			expectedError: errors.New("not authenticated"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Patient",
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
			resourceType: "Patient",
			name:         "fhirclient error",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			errorFromSearch: errors.New("error"),
			expectedError:   errors.New("error"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Patient",
			name:         "Patient returned, error from CarePlan read",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			errorFromCarePlanRead: errors.New("error"),
			expectedError:         errors.New("error"),
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Patient",
			name:         "Patient returned, no careplan or careteam returned",
			searchParams: url.Values{},
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
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			resourceType: "Patient",
			name:         "Patient returned, careplan and careteam returned, incorrect principal",
			searchParams: url.Values{},
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
						Resource: careplan1Careteam2,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Patient",
			name:         "Patient returned, careplan returned, correct principal",
			searchParams: url.Values{},
			returnedBundle: &fhir.Bundle{
				Link: []fhir.BundleLink{
					{
						Relation: "self",
						Url:      "http://example.com/fhir/Patient?some-query-params",
					},
				},
				Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: careplan1Careteam2,
					},
				},
			},
			expectedBundle: &fhir.Bundle{
				Link: []fhir.BundleLink{
					{
						Relation: "self",
						Url:      "http://example.com/fhir/Patient?some-query-params",
					},
				},
				Timestamp: to.Ptr("2021-09-01T12:00:00Z"),
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1,
					},
				},
			},
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			resourceType: "Patient",
			name:         "Multiple resources returned, correctly filtered",
			searchParams: url.Values{},
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
						Resource: careplan1Careteam2,
					},
					{
						Resource: careplan2Careteam1,
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
			testHelperHandleSearchResource[fhir.Patient](t, tt, service.handleSearchPatient)
		})
	}
}
