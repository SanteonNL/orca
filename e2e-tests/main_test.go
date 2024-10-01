package main

import (
	"e2e-tests/to"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

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

	// Setup Clinic
	const clinicFHIRStoreURL = "http://fhirstore:8080/fhir/clinic"
	const clinicBaseUrl = "http://clinic-orchestrator:8080"
	const carePlanServiceBaseURL = clinicBaseUrl + "/cps"
	err = createTenant(nutsInternalURL, hapiFhirClient, "clinic", 1, "Clinic", "Bug City", clinicBaseUrl+"/cpc/fhir/notify")
	require.NoError(t, err)
	setupOrchestrator(t, dockerNetwork.Name, "clinic-orchestrator", "clinic", true, carePlanServiceBaseURL, clinicFHIRStoreURL)

	// Setup Hospital
	const hospitalFHIRStoreURL = "http://fhirstore:8080/fhir/hospital"
	const hospitalBaseUrl = "http://clinic-orchestrator:8080"
	err = createTenant(nutsInternalURL, hapiFhirClient, "hospital", 2, "Hospital", "Fix City", hospitalBaseUrl+"/cpc/fhir/notify")
	require.NoError(t, err)
	hospitalOrcaURL := setupOrchestrator(t, dockerNetwork.Name, "hospital-orchestrator", "hospital", true, carePlanServiceBaseURL, hospitalFHIRStoreURL)
	hospitalOrcaFHIRClient := fhirclient.New(hospitalOrcaURL.JoinPath("/cpc/cps/fhir"), orcaHttpClient, nil)

	t.Run("EHR using Orchestrator REST API", func(t *testing.T) {
		t.Run("Hospital EHR creates New CarePlan, New Task", func(t *testing.T) {
			t.Log("Creating new CarePlan...")
			carePlan := fhir.CarePlan{}
			{
				err := hospitalOrcaFHIRClient.Create(carePlan, &carePlan)
				require.NoError(t, err)
			}
			t.Log("Creating new Task...")
			var task fhir.Task
			{
				task.Intent = "order"
				task.Status = fhir.TaskStatusRequested
				task.BasedOn = []fhir.Reference{
					{
						Reference: to.Ptr("CarePlan/" + *carePlan.Id),
					},
				}
				err := hospitalOrcaFHIRClient.Create(task, &task)
				require.NoError(t, err)
			}
			t.Skip("TODO: Add Questionnaire dance")
		})
	})
	t.Run("ORCA Frontend using Orchestrator REST API", func(t *testing.T) {
		t.Skip("TODO")
	})
}
