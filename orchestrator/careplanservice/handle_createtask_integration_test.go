package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
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

func Test_Integration_Service_handleCreateTask(t *testing.T) {
	fhirBaseURL := setupHAPI(t)

	config := DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	service, err := New(config, nil)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")
	carePlanContributor := fhirclient.New(carePlanServiceURL, httpService.Client(), nil)

	t.Run("New CarePlan, New Task", func(t *testing.T) {
		var carePlan fhir.CarePlan
		t.Run("Create CarePlan", func(t *testing.T) {
			carePlan.Subject = fhir.Reference{
				Type: to.Ptr("Patient"),
				Identifier: &fhir.Identifier{
					System: to.Ptr("bsn"), // TODO: proper URA system
					Value:  to.Ptr("123456789"),
				},
			}
			err := carePlanContributor.Create(carePlan, &carePlan)
			require.NoError(t, err)
		})
		task := fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
		}
		err := carePlanContributor.Create(task, &task)
		require.NoError(t, err)

		t.Run("Check Task properties", func(t *testing.T) {
			data, _ := json.MarshalIndent(task, "", "  ")
			println(string(data))
			require.NotNil(t, task.Id)
			require.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference, "Task.BasedOn should reference CarePlan")
		})
		t.Run("Check that CarePlan.activities contains the Task", func(t *testing.T) {
			err = carePlanContributor.Read("CarePlan/"+*carePlan.Id, &carePlan)
			require.NoError(t, err)
			require.Len(t, carePlan.Activity, 1)
			require.Equal(t, "Task", *carePlan.Activity[0].Reference.Type)
			require.Equal(t, "Task/"+*task.Id, *carePlan.Activity[0].Reference.Reference)
			data, _ := json.MarshalIndent(carePlan, "", "  ")
			println(string(data))
		})
	})
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
