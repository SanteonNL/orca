package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"os"
	"reflect"
	"testing"
)

type TestStruct[T any] struct {
	ctx                    context.Context
	name                   string
	id                     string
	resourceType           string
	returnedResource       *T
	errorFromRead          error
	returnedCarePlanBundle *fhir.Bundle
	errorFromCarePlanRead  error
	// For resources that require a Task Search to validate
	returnedTaskBundle      *fhir.Bundle
	errorFromTaskBundleRead error
	// For resources that are part of a task
	returnedTaskId    string
	returnedTask      *fhir.Task
	errorFromTaskRead error
	// For resources that require a Patient Search to validate
	returnedPatientBundle      *fhir.Bundle
	errorFromPatientBundleRead error
	expectError                bool
}

// We have many resources that have similar Get requirements, all tests with at least one of the following requirements can go here:
// * Requester must be authorised
// * Requester must be a member of the CareTeam being requested
// * The CareTeam must be associated with the CarePlan to which the resource belongs.
func Test_handleGetResource(t *testing.T) {
	task1Raw, _ := os.ReadFile("./testdata/task-1.json")
	var task1 fhir.Task
	_ = json.Unmarshal(task1Raw, &task1)
	carePlan1Raw, _ := os.ReadFile("./testdata/careplan-1.json")
	var carePlan1 fhir.CarePlan
	_ = json.Unmarshal(carePlan1Raw, &carePlan1)
	careTeam2Raw, _ := os.ReadFile("./testdata/careteam-2.json")
	var careTeam2 fhir.CareTeam
	_ = json.Unmarshal(careTeam2Raw, &careTeam2)

	patient1Raw, _ := os.ReadFile("./testdata/patient-1.json")
	var patient1 fhir.Patient
	_ = json.Unmarshal(patient1Raw, &patient1)

	taskTests := []TestStruct[fhir.Task]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "Task",
			expectError:  true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "Task does not exist",
			id:               "1",
			resourceType:     "Task",
			returnedResource: nil,
			errorFromRead:    errors.New("error"),
			expectError:      true,
		},
		{
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:                   "Task exists, auth, not owner or requester, error fetching CarePlan",
			id:                     "1",
			resourceType:           "Task",
			returnedResource:       &task1,
			returnedCarePlanBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			errorFromCarePlanRead:  errors.New("error"),
			expectError:            true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "Task exists, auth, CarePlan and CareTeam returned, not a participant",
			id:               "1",
			resourceType:     "Task",
			returnedResource: &task1,
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
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:             "Task exists, auth, CarePlan and CareTeam returned, owner",
			id:               "1",
			resourceType:     "Task",
			returnedResource: &task1,
			expectError:      false,
		},
	}
	carePlanTests := []TestStruct[fhir.CarePlan]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "CarePlan",
			expectError:  true,
		},
		{
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:                   "CarePlan does not exist",
			id:                     "1",
			resourceType:           "CarePlan",
			errorFromRead:          errors.New("error"),
			returnedCarePlanBundle: &fhir.Bundle{Entry: []fhir.BundleEntry{}},
			expectError:            true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "No CareTeams returned",
			id:           "1",
			resourceType: "CarePlan",
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
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "CarePlan, CareTeam returned, incorrect principal",
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
			expectError: true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "CarePlan, CareTeam returned, correct principal",
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
			expectError:      false,
			returnedResource: &carePlan1,
		},
	}
	careTeamTests := []TestStruct[fhir.CareTeam]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "2",
			resourceType: "CareTeam",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:          "CareTeam does not exist",
			id:            "2",
			resourceType:  "CareTeam",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:             "CareTeam exists, auth, incorrect principal",
			id:               "2",
			resourceType:     "CareTeam",
			returnedResource: &careTeam2,
			expectError:      true,
		},
		{
			ctx:              auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:             "CareTeam exists, auth, correct principal",
			id:               "2",
			resourceType:     "CareTeam",
			returnedResource: &careTeam2,
			expectError:      false,
		},
	}
	patientTests := []TestStruct[fhir.Patient]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "Patient",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "Patient does not exist",
			id:            "1",
			resourceType:  "Patient",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:                   auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:                  "Patient exists, auth, error fetching CarePlan",
			id:                    "1",
			resourceType:          "Patient",
			errorFromCarePlanRead: errors.New("error"),
			expectError:           true,
			returnedResource:      &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Patient exists, auth, No CarePlans returned",
			id:           "1",
			resourceType: "Patient",
			returnedCarePlanBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			},
			expectError:      true,
			returnedResource: &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "Patient exists, auth, CarePlan and CareTeam returned, not a participant",
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
			expectError:      true,
			returnedResource: &patient1,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Patient exists, auth, CarePlan and CareTeam returned, correct principal",
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
			expectError:      false,
			returnedResource: &patient1,
		},
	}
	questionnaireTests := []TestStruct[fhir.Questionnaire]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "Questionnaire",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "Questionnaire does not exist",
			id:            "1",
			resourceType:  "Questionnaire",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Questionnaire exists, auth",
			id:           "1",
			resourceType: "Questionnaire",
			expectError:  false,
			returnedResource: &fhir.Questionnaire{
				Id: to.Ptr("1"),
			},
		},
	}
	questionnaireResponseTests := []TestStruct[fhir.QuestionnaireResponse]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "QuestionnaireResponse does not exist",
			id:            "1",
			resourceType:  "QuestionnaireResponse",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "QuestionnaireResponse exists, no basedOn",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
			},
			expectError: true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "QuestionnaireResponse exists, error fetching task",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId:    "1",
			errorFromTaskRead: errors.New("error"),
			expectError:       true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "QuestionnaireResponse exists, fetched task, incorrect principal",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId: "1",
			returnedTask:   &task1,
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
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "QuestionnaireResponse exists, fetched task, task owner",
			id:           "1",
			resourceType: "QuestionnaireResponse",
			returnedResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("Task/1"),
					},
				},
			},
			returnedTaskId: "1",
			returnedTask:   &task1,
			expectError:    false,
		},
	}
	serviceRequestTests := []TestStruct[fhir.ServiceRequest]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "ServiceRequest",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "ServiceRequest does not exist",
			id:            "1",
			resourceType:  "ServiceRequest",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ServiceRequest exists, error searching for task",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			errorFromTaskBundleRead: errors.New("error"),
			expectError:             true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "ServiceRequest exists, fetched task, incorrect principal",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			returnedTaskBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1Raw,
					},
				},
			},
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
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "ServiceRequest exists, fetched task, task owner",
			id:           "1",
			resourceType: "ServiceRequest",
			returnedResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			returnedTaskBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: task1Raw,
					},
				},
			},
			expectError: false,
		},
	}
	conditionTests := []TestStruct[fhir.Condition]{
		{
			ctx:          context.Background(),
			name:         "No auth",
			id:           "1",
			resourceType: "Condition",
			expectError:  true,
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "Condition does not exist",
			id:            "1",
			resourceType:  "Condition",
			errorFromRead: errors.New("error"),
			expectError:   true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Condition exists, no subject",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
			},
			expectError: true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Condition exists, subject is not a patient",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("SomethingWrong"),
						Value:  to.Ptr("1"),
					},
				},
			},
			expectError: true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Condition exists, subject is patient, error fetching patient",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
			errorFromPatientBundleRead: errors.New("error"),
			expectError:                true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal3),
			name:         "Condition exists, subject is patient, patient returned, incorrect principal",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
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
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1Raw,
					},
				},
			},
			expectError: true,
		},
		{
			ctx:          auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:         "Condition exists, subject is patient, patient returned, correct principal",
			id:           "1",
			resourceType: "Condition",
			returnedResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Subject: fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("123456789"),
					},
				},
			},
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
			returnedPatientBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: patient1Raw,
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range taskTests {
		testHelperHandleGetResource[fhir.Task](t, tt)
	}
	for _, tt := range carePlanTests {
		testHelperHandleGetResource[fhir.CarePlan](t, tt)
	}
	for _, tt := range careTeamTests {
		testHelperHandleGetResource[fhir.CareTeam](t, tt)
	}
	for _, tt := range patientTests {
		testHelperHandleGetResource[fhir.Patient](t, tt)
	}
	for _, tt := range questionnaireTests {
		testHelperHandleGetResource[fhir.Questionnaire](t, tt)
	}
	for _, tt := range questionnaireResponseTests {
		testHelperHandleGetResource[fhir.QuestionnaireResponse](t, tt)
	}
	for _, tt := range serviceRequestTests {
		testHelperHandleGetResource[fhir.ServiceRequest](t, tt)
	}
	for _, tt := range conditionTests {
		testHelperHandleGetResource[fhir.Condition](t, tt)
	}
}

func testHelperHandleGetResource[T any](t *testing.T, params TestStruct[T]) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	t.Run(fmt.Sprintf("Test %s: %s", params.resourceType, params.name), func(t *testing.T) {
		if params.returnedCarePlanBundle != nil || params.errorFromCarePlanRead != nil {
			mockFHIRClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedCarePlanBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedCarePlanBundle))
				}
				return params.errorFromCarePlanRead
			})
		}
		if params.returnedTaskBundle != nil || params.errorFromTaskBundleRead != nil {
			mockFHIRClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedTaskBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedTaskBundle))
				}
				return params.errorFromTaskBundleRead
			})
		}
		if params.returnedTask != nil || params.errorFromTaskRead != nil {
			mockFHIRClient.EXPECT().Read("Task/"+params.returnedTaskId, gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedTask != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedTask))
				}
				return params.errorFromTaskRead
			})
		}
		if params.returnedPatientBundle != nil || params.errorFromPatientBundleRead != nil {
			mockFHIRClient.EXPECT().Read("Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedPatientBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedPatientBundle))
				}
				return params.errorFromPatientBundleRead
			})
		}
		if (params.returnedResource != nil || params.errorFromRead != nil) && params.resourceType != "CarePlan" {
			mockFHIRClient.EXPECT().Read(params.resourceType+"/"+params.id, gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedResource != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedResource))
				}
				return params.errorFromRead
			})
		}
		var handler func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error)
		switch params.resourceType {
		case "Task":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetTask(ctx, id, headers)
			}
		case "CarePlan":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetCarePlan(ctx, id, headers)
			}
		case "CareTeam":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetCareTeam(ctx, id, headers)
			}
		case "Patient":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetPatient(ctx, id, headers)
			}
		case "Questionnaire":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetQuestionnaire(ctx, id, headers)
			}
		case "QuestionnaireResponse":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetQuestionnaireResponse(ctx, id, headers)
			}
		case "ServiceRequest":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetServiceRequest(ctx, id, headers)
			}
		case "Condition":
			handler = func(ctx context.Context, id string, headers *fhirclient.Headers) (interface{}, error) {
				return service.handleGetCondition(ctx, id, headers)
			}
		}

		got, err := handler(params.ctx, params.id, &fhirclient.Headers{})
		if params.expectError == true {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, params.returnedResource, got)
		}
	})
}

// TODO: Write generic handler for Search
