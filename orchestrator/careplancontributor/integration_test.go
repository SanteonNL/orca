package careplancontributor

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func Test_Integration_CPCFHIRProxy(t *testing.T) {
	cpsDataRequester, cpcDataRequester, cpsDataHolder := setupIntegrationTest(t)

	var carePlan fhir.CarePlan
	var task fhir.Task
	t.Log("Creating CarePlan...")
	{
		carePlan.Subject = fhir.Reference{
			Type: to.Ptr("Patient"),
			Identifier: &fhir.Identifier{
				System: to.Ptr(coolfhir.BSNNamingSystem),
				Value:  to.Ptr("123456789"),
			},
		}
		err := cpsDataHolder.Create(carePlan, &carePlan)
		require.NoError(t, err)

		// Check the CarePlan and CareTeam exist
		var createdCarePlan fhir.CarePlan
		var createdCareTeams []fhir.CareTeam
		err = cpsDataHolder.Read("CarePlan/"+*carePlan.Id, &createdCarePlan, fhirclient.ResolveRef("careTeam", &createdCareTeams))
		require.NoError(t, err)
		require.NotNil(t, createdCarePlan.Id)
		require.Len(t, createdCareTeams, 1, "expected 1 CareTeam")
		require.NotNil(t, createdCareTeams[0].Id)
	}

	t.Log("Creating Task")
	{
		task = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}

		err := cpsDataHolder.Create(task, &task)
		require.NoError(t, err)
		println("actual care plan ID: " + *carePlan.Id)
		err = cpsDataHolder.Read("CarePlan/"+*carePlan.Id, &carePlan)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, task.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			require.Len(t, carePlan.Activity, 1)
			require.Equal(t, "Task", *carePlan.Activity[0].Reference.Type)
			require.Equal(t, "Task/"+*task.Id, *carePlan.Activity[0].Reference.Reference)
		})
	}
	t.Log("Reading task before accepted - Fails")
	{
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Accepting Task")
	{
		task.Status = fhir.TaskStatusAccepted
		var updatedTask fhir.Task
		err := cpsDataRequester.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusAccepted, updatedTask.Status)
		})
	}
	t.Log("Reading task after accepted")
	{
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.NoError(t, err)
	}
	// TODO: Failure case tests
}

func setupIntegrationTest(t *testing.T) (*fhirclient.BaseClient, *fhirclient.BaseClient, *fhirclient.BaseClient) {
	fhirBaseURL := setupHAPI(t)
	config := careplanservice.DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	service, err := careplanservice.New(config, profile.TestProfile{}, orcaPublicURL)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")

	println("xSCP header: " + carePlanServiceURL.String() + "/CarePlan/1")

	transport1 := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal1, "")
	transport2 := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/1")

	cpsDataHolder := fhirclient.New(carePlanServiceURL, &http.Client{Transport: transport1}, nil)
	cpsDataRequester := fhirclient.New(carePlanServiceURL, &http.Client{Transport: transport2}, nil)

	cpcConfig := DefaultConfig()
	cpcConfig.Enabled = true
	cpcConfig.FHIR.BaseURL = fhirBaseURL.String()
	cpcConfig.CarePlanService.URL = carePlanServiceURL.String()
	sessionManager, _ := createTestSession()
	cpc, err := New(cpcConfig, profile.TestProfile{}, orcaPublicURL, sessionManager)
	require.NoError(t, err)

	cpcServerMux := http.NewServeMux()
	cpcHttpService := httptest.NewServer(cpcServerMux)
	cpc.RegisterHandlers(cpcServerMux)
	cpcURL, _ := url.Parse(cpcHttpService.URL + "/contrib/fhir")

	cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: transport2}, nil)

	return cpsDataRequester, cpcDataRequester, cpsDataHolder
}

func setupHAPI(t *testing.T) *url.URL {
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "hapiproject/hapi:v7.2.0",
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"hapi.fhir.fhir_version": "R4",
		},
		WaitingFor: wait.ForHTTP("/fhir/Task"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			panic(err)
		}
	})
	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(t, err)
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	return u.JoinPath("fhir")
}
