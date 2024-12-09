package main

import (
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

const URANamingSystem = "http://fhir.nl/fhir/NamingSystem/ura"

func Test_Main(t *testing.T) {
	dockerNetwork, err := setupDockerNetwork(t)
	// Setup HAPI FHIR server
	hapiBaseURL := setupHAPI(t, dockerNetwork.Name)
	hapiFhirClient := fhirclient.New(hapiBaseURL, http.DefaultClient, nil)
	// Setup Nuts node
	_, nutsInternalURL := setupNutsNode(t, dockerNetwork.Name)
	orcaHttpClient := &http.Client{
		Transport: &AuthorizedRoundTripper{
			Value:      "Bearer valid",
			Underlying: http.DefaultTransport,
		},
	}

	const clinicFHIRStoreURL = "http://fhirstore:8080/fhir/clinic"
	const clinicQuestionnaireFHIRStoreURL = "http://fhirstore:8080/fhir/DEFAULT" // HAPI only allows Questionnaires in the default partition
	const clinicBaseUrl = "http://clinic-orchestrator:8080"
	const clinicURA = 1

	const hospitalFHIRStoreURL = "http://fhirstore:8080/fhir/DEFAULT"
	const hospitalBaseUrl = "http://hospital-orchestrator:8080"
	const hospitalURA = 2

	const carePlanServiceBaseURL = hospitalBaseUrl + "/cps"

	// Setup Clinic
	err = createTenant(nutsInternalURL, hapiFhirClient, "clinic", clinicURA, "Clinic", "Bug City", clinicBaseUrl+"/cpc/fhir/notify", false)
	require.NoError(t, err)
	clinicOrcaURL := setupOrchestrator(t, dockerNetwork.Name, "clinic-orchestrator", "clinic", false, carePlanServiceBaseURL, clinicFHIRStoreURL, clinicQuestionnaireFHIRStoreURL, true)
	clinicOrcaFHIRClient := fhirclient.New(clinicOrcaURL.JoinPath("/cpc/cps/fhir"), orcaHttpClient, nil)

	// Setup Hospital
	// Questionnaires can't be created in HAPI FHIR server partitions, only in the default partition.
	// Otherwise, the following error occurs: HAPI-1318: Resource type Questionnaire can not be partitioned
	// This is why the hospital, running the CPS, stores its data in the default partition.
	err = createTenant(nutsInternalURL, hapiFhirClient, "hospital", hospitalURA, "Hospital", "Fix City", hospitalBaseUrl+"/cpc/fhir/notify", true)
	require.NoError(t, err)
	hospitalOrcaURL := setupOrchestrator(t, dockerNetwork.Name, "hospital-orchestrator", "hospital", true, carePlanServiceBaseURL, hospitalFHIRStoreURL, "", true)
	// hospitalOrcaFHIRClient is the FHIR client the hospital uses to interact with the CarePlanService
	hospitalOrcaFHIRClient := fhirclient.New(hospitalOrcaURL.JoinPath("/cpc/cps/fhir"), orcaHttpClient, nil)

	var patient fhir.Patient
	var task fhir.Task
	var serviceRequest fhir.ServiceRequest
	t.Run("EHR using Orchestrator REST API", func(t *testing.T) {
		t.Log("Creating patient for Task to refer to")
		{
			patient = fhir.Patient{
				Meta: &fhir.Meta{
					Profile: []string{
						"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCP-Patient",
					},
				},
				Identifier: []fhir.Identifier{
					{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			}
			err := hospitalOrcaFHIRClient.Create(patient, &patient)
			require.NoError(t, err)
		}
		t.Run("Hospital EHR creates new Task", func(t *testing.T) {
			t.Log("Creating new Task...")
			{
				t.Log("  Creating associated ServiceRequest...")
				serviceRequest = fhir.ServiceRequest{
					Meta: &fhir.Meta{
						Profile: []string{
							"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
						},
					},
					Code: &fhir.CodeableConcept{
						Coding: []fhir.Coding{
							{
								System: to.Ptr("http://snomed.info/sct"),
								Code:   to.Ptr("719858009"), // Telemonitoring
							},
						},
					},
				}
				err := hospitalOrcaFHIRClient.Create(serviceRequest, &serviceRequest)
				require.NoError(t, err)

				task.Meta = &fhir.Meta{
					Profile: []string{
						"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
					},
				}
				task.Requester = &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr(URANamingSystem),
						Value:  to.Ptr(strconv.Itoa(hospitalURA)),
					},
					Type: to.Ptr("Organization"),
				}
				task.Owner = &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr(URANamingSystem),
						Value:  to.Ptr(strconv.Itoa(clinicURA)),
					},
					Type: to.Ptr("Organization"),
				}
				task.Focus = &fhir.Reference{
					Reference: to.Ptr("ServiceRequest/" + *serviceRequest.ID),
				}
				task.ReasonCode = &fhir.CodeableConcept{
					Coding: []fhir.Coding{
						{
							System: to.Ptr("http://snomed.info/sct"),
							Code:   to.Ptr("84114007"), // Heart failure
						},
					},
				}
				task.For = &fhir.Reference{
					Identifier: &fhir.Identifier{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				}
				task.Intent = "order"
				task.Status = fhir.TaskStatusRequested
				err = hospitalOrcaFHIRClient.Create(task, &task)
				require.NoError(t, err)
			}
			t.Log("Responding to Task Questionnaire")
			{
				var searchResult fhir.Bundle
				err = hospitalOrcaFHIRClient.Search("Task", url.Values{"part-of": {"Task/" + *task.ID}}, &searchResult)
				require.NoError(t, err)
				require.Len(t, searchResult.Entry, 1, "Expected 1 subtask")

				var subTask fhir.Task
				// Assert subtask with Questionnaire
				var questionnaireRef string
				{
					require.NoError(t, json.Unmarshal(searchResult.Entry[0].Resource, &subTask))
					require.Len(t, subTask.Input, 1, "Expected 1 input")
					require.NotNil(t, subTask.Input[0].ValueReference, "Expected input valueReference")
					require.NotNil(t, subTask.Input[0].ValueReference.Reference, "Expected input valueReference reference")
					questionnaireRef = *subTask.Input[0].ValueReference.Reference
					require.True(t, strings.HasPrefix(questionnaireRef, "Questionnaire/"), "Expected input valueReference reference to start with 'Questionnaire/'")
					require.NoError(t, hospitalOrcaFHIRClient.Read(questionnaireRef, &fhir.Questionnaire{}))
				}
				questionnaireResponse := questionnaireResponseTo(questionnaireRef)
				subTask.Status = fhir.TaskStatusCompleted
				subTask.Output = append(subTask.Output, fhir.TaskOutput{
					Type: fhir.CodeableConcept{
						Coding: []fhir.Coding{
							{
								System: to.Ptr("http://terminology.hl7.org/CodeSystem/task-output-type"),
								Code:   to.Ptr("Reference"),
							},
						},
					},
					ValueReference: &fhir.Reference{
						Reference: to.Ptr("urn:uuid:questionnaire-response"),
					},
				})
				responseBundle := caramel.Transaction().
					Create(questionnaireResponse, caramel.WithFullUrl("urn:uuid:questionnaire-response")).
					Update(subTask, "Task/"+*subTask.ID).Bundle()
				err = hospitalOrcaFHIRClient.Create(responseBundle, &responseBundle, fhirclient.AtPath("/"))
				require.NoError(t, err)

				// Get QuestionnaireResponse ID from Bundle
				err = json.Unmarshal(responseBundle.Entry[0].Resource, &questionnaireResponse)
				require.NoError(t, err)

				// Get QuestionnaireResponse, which will use the custom SearchParameter to verify the user has access
				var fetchedQuestionnaireResponse fhir.QuestionnaireResponse
				err = hospitalOrcaFHIRClient.Read("QuestionnaireResponse/"+*questionnaireResponse.ID, &fetchedQuestionnaireResponse)
				require.NoError(t, err)
				require.Equal(t, *questionnaireResponse.ID, *fetchedQuestionnaireResponse.ID)
				require.Equal(t, *questionnaireResponse.Questionnaire, *fetchedQuestionnaireResponse.Questionnaire)
				require.Equal(t, questionnaireResponse.Status, fetchedQuestionnaireResponse.Status)
				require.Equal(t, len(questionnaireResponse.Item), len(fetchedQuestionnaireResponse.Item))
			}

			//t.Log("Filler adding Questionnaire sub-Task...")
			//subTask := fhir.Task{}
			//{
			//
			//}
		})
	})
	t.Run("Clinic attempts to create a CarePlan at Hospital's CarePlanService, which isn't allowed", func(t *testing.T) {
		var task fhir.Task
		t.Log("Clinic attempts to create task without existing CarePlan in clinic, fails...")
		{
			task.Meta = &fhir.Meta{
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			}
			task.Requester = &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(URANamingSystem),
					Value:  to.Ptr(strconv.Itoa(hospitalURA)),
				},
				Type: to.Ptr("Organization"),
			}
			task.Owner = &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(URANamingSystem),
					Value:  to.Ptr(strconv.Itoa(clinicURA)),
				},
				Type: to.Ptr("Organization"),
			}
			task.Focus = &fhir.Reference{
				Identifier: &fhir.Identifier{
					// COPD
					System: to.Ptr("2.16.528.1.1007.3.3.21514.ehr.orders"),
					Value:  to.Ptr("99534756439"),
				},
			}
			task.For = &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
				Reference: to.Ptr("Patient/" + *patient.ID),
			}
			task.Intent = "order"
			task.Status = fhir.TaskStatusRequested
			err := clinicOrcaFHIRClient.Create(task, &task)
			var operationOutcome fhirclient.OperationOutcomeError
			require.ErrorAs(t, err, &operationOutcome)
			require.Len(t, operationOutcome.Issue, 1)
			require.Equal(t, "CarePlanService/CreateTask failed: requester must be local care organization in order to create new CarePlan and CareTeam", *operationOutcome.Issue[0].Diagnostics)
		}
	})
	t.Run("Test resource GET authorisation", func(t *testing.T) {
		// TODO: Negative testing with a third party that has a valid bearer token but no access to the existing CarePlan and CareTeams
		// Patient
		var fetchedPatient fhir.Patient
		err = hospitalOrcaFHIRClient.Read("Patient/"+*patient.ID, &fetchedPatient)
		require.NoError(t, err)
		require.Equal(t, *patient.ID, *fetchedPatient.ID)
		require.Equal(t, *patient.Identifier[0].Value, *fetchedPatient.Identifier[0].Value)

		err = clinicOrcaFHIRClient.Read("Patient/"+*patient.ID, &fetchedPatient)
		require.NoError(t, err)
		require.Equal(t, *patient.ID, *fetchedPatient.ID)
		require.Equal(t, *patient.Identifier[0].Value, *fetchedPatient.Identifier[0].Value)

		// ServiceRequest
		var fetchedServiceRequest fhir.ServiceRequest
		err = hospitalOrcaFHIRClient.Read("ServiceRequest/"+*serviceRequest.ID, &fetchedServiceRequest)
		require.NoError(t, err)
		require.Equal(t, *serviceRequest.ID, *fetchedServiceRequest.ID)
		require.Equal(t, *serviceRequest.Code.Coding[0].Code, *fetchedServiceRequest.Code.Coding[0].Code)

		err = clinicOrcaFHIRClient.Read("ServiceRequest/"+*serviceRequest.ID, &fetchedServiceRequest)
		require.NoError(t, err)
		require.Equal(t, *serviceRequest.ID, *fetchedServiceRequest.ID)
		require.Equal(t, *serviceRequest.Code.Coding[0].Code, *fetchedServiceRequest.Code.Coding[0].Code)
	})
	t.Run("Task Filler doesn't support the ServiceRequest code, and rejects the Task", func(t *testing.T) {
		unsupportedServiceRequest := fhir.ServiceRequest{
			Meta: &fhir.Meta{
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			},
			Code: &fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System: to.Ptr("http://snomed.info/sct"),
						Code:   to.Ptr("1234"), // not supported by Task Filler
					},
				},
			},
		}
		err := hospitalOrcaFHIRClient.Create(unsupportedServiceRequest, &unsupportedServiceRequest)
		require.NoError(t, err)

		unsupportedTask := fhir.Task{
			Meta: &fhir.Meta{
				Profile: []string{
					"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
				},
			},
			Requester: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(URANamingSystem),
					Value:  to.Ptr(strconv.Itoa(hospitalURA)),
				},
			},
			Owner: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(URANamingSystem),
					Value:  to.Ptr(strconv.Itoa(clinicURA)),
				},
			},
			Focus: &fhir.Reference{
				Reference: to.Ptr("ServiceRequest/" + *unsupportedServiceRequest.ID),
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			ReasonCode: &fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System: to.Ptr("http://snomed.info/sct"),
						Code:   to.Ptr("13645005"), // COPD
					},
				},
			},
			Intent: "order",
			Status: fhir.TaskStatusRequested,
		}
		err = hospitalOrcaFHIRClient.Create(unsupportedTask, &unsupportedTask)
		require.NoError(t, err)

		t.Run("assert Task is rejected", func(t *testing.T) {
			var rejectedTask fhir.Task
			err = hospitalOrcaFHIRClient.Read("Task/"+*unsupportedTask.ID, &rejectedTask)
			require.NoError(t, err)
			require.Equal(t, fhir.TaskStatusRejected, rejectedTask.Status)
		})
	})
}
