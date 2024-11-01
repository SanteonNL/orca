package main

import (
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
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
	const clinicBaseUrl = "http://clinic-orchestrator:8080"
	const clinicURA = 1

	const hospitalFHIRStoreURL = "http://fhirstore:8080/fhir/DEFAULT"
	const hospitalBaseUrl = "http://hospital-orchestrator:8080"
	const hospitalURA = 2

	const carePlanServiceBaseURL = hospitalBaseUrl + "/cps"

	// Setup Clinic
	err = createTenant(nutsInternalURL, hapiFhirClient, "clinic", clinicURA, "Clinic", "Bug City", clinicBaseUrl+"/cpc/fhir/notify", false)
	require.NoError(t, err)
	clinicOrcaURL := setupOrchestrator(t, dockerNetwork.Name, "clinic-orchestrator", "clinic", false, carePlanServiceBaseURL, clinicFHIRStoreURL)
	clinicOrcaFHIRClient := fhirclient.New(clinicOrcaURL.JoinPath("/cpc/cps/fhir"), orcaHttpClient, nil)

	// Setup Hospital
	// Questionnaires can't be created in HAPI FHIR server partitions, only in the default partition.
	// Otherwise, the following error occurs: HAPI-1318: Resource type Questionnaire can not be partitioned
	// This is why the hospital, running the CPS, stores its data in the default partition.
	err = createTenant(nutsInternalURL, hapiFhirClient, "hospital", hospitalURA, "Hospital", "Fix City", hospitalBaseUrl+"/cpc/fhir/notify", true)
	require.NoError(t, err)
	hospitalOrcaURL := setupOrchestrator(t, dockerNetwork.Name, "hospital-orchestrator", "hospital", true, carePlanServiceBaseURL, hospitalFHIRStoreURL)
	// hospitalOrcaFHIRClient is the FHIR client the hospital uses to interact with the CarePlanService
	hospitalOrcaFHIRClient := fhirclient.New(hospitalOrcaURL.JoinPath("/cpc/cps/fhir"), orcaHttpClient, nil)

	t.Run("EHR using Orchestrator REST API", func(t *testing.T) {
		t.Run("Hospital EHR creates new Task", func(t *testing.T) {
			var task fhir.Task
			t.Log("Creating new Task...")
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
				}
				task.Intent = "order"
				task.Status = fhir.TaskStatusRequested
				err := hospitalOrcaFHIRClient.Create(task, &task)
				require.NoError(t, err)
			}
			t.Log("Responding to Task Questionnaire")
			{
				var searchResult fhir.Bundle
				err = hospitalOrcaFHIRClient.Read("Task", &searchResult, fhirclient.QueryParam("part-of", "Task/"+*task.ID))
				require.NoError(t, err)
				require.Len(t, searchResult.Entry, 1, "Expected 1 subtask")

				var subTask fhir.Task
				// Assert subtask with Questionnaire
				var questionnaire fhir.Questionnaire
				{
					require.NoError(t, json.Unmarshal(searchResult.Entry[0].Resource, &subTask))
					require.Len(t, subTask.Input, 1, "Expected 1 input")
					require.NotNil(t, subTask.Input[0].ValueReference, "Expected input valueReference")
					require.NotNil(t, subTask.Input[0].ValueReference.Reference, "Expected input valueReference reference")
					questionnaireRef := *subTask.Input[0].ValueReference.Reference
					require.True(t, strings.HasPrefix(questionnaireRef, "Questionnaire/"), "Expected input valueReference reference to start with 'Questionnaire/'")
					err = hospitalOrcaFHIRClient.Read(questionnaireRef, &questionnaire)
					require.NoError(t, err)
				}
				questionnaireResponse := questionnaireResponseTo(questionnaire)
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
			}
			task.Intent = "order"
			task.Status = fhir.TaskStatusRequested
			err := clinicOrcaFHIRClient.Create(task, &task)
			var operationOutcome fhirclient.OperationOutcome
			require.ErrorAs(t, err, &operationOutcome)
			require.Len(t, operationOutcome.Issue, 1)
			require.Equal(t, "CarePlanService/CreateTask failed: requester must be local care organization in order to create new CarePlan and CareTeam", *operationOutcome.Issue[0].Diagnostics)
		}
	})
}
