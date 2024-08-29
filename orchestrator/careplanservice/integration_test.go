package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func Test_Integration_TaskLifecycle(t *testing.T) {
	// Note: this test consists of multiple steps that look like subtests, but they can't be subtests:
	//       in Golang, running a single Subtest causes the other tests not to run.
	//       This causes issues, since each test step (e.g. accepting Task) requires the previous step (test) to succeed (e.g. creating Task).
	t.Log("This test requires creates a new CarePlan and Task, then runs the Task through requested->accepted->completed lifecycle.")
	carePlanContributor := setupIntegrationTest(t)

	participant1 := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
		Period:     &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2 := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period:     &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	}
	participant2WithEndDate := fhir.CareTeamParticipant{
		OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		Period: &fhir.Period{
			Start: to.Ptr("2021-01-01T00:00:00Z"),
			End:   to.Ptr("2021-01-02T00:00:00Z"),
		},
	}

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
		err := carePlanContributor.Create(carePlan, &carePlan)
		require.NoError(t, err)

		// Check the CarePlan and CareTeam exist
		var createdCarePlan fhir.CarePlan
		var createdCareTeams []fhir.CareTeam
		err = carePlanContributor.Read("CarePlan/"+*carePlan.Id, &createdCarePlan, fhirclient.ResolveRef("careTeam", &createdCareTeams))
		require.NoError(t, err)
		require.NotNil(t, createdCarePlan.Id)
		require.Len(t, createdCareTeams, 1, "expected 1 CareTeam")
		require.NotNil(t, createdCareTeams[0].Id)
	}

	t.Log("Creating Task - Invalid status")
	{
		task = fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/" + *carePlan.Id),
				},
			},
			Status:    fhir.TaskStatusAccepted,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}

		err := carePlanContributor.Create(task, &task)
		require.Error(t, err)
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

		err := carePlanContributor.Create(task, &task)
		require.NoError(t, err)
		err = carePlanContributor.Read("CarePlan/"+*carePlan.Id, &carePlan)
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
		t.Run("Check that CareTeam now contains the requesting party", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor, *carePlan.CareTeam[0].Reference, participant1)
		})
	}

	t.Log("Accepting Task")
	{
		task.Status = fhir.TaskStatusAccepted
		var updatedTask fhir.Task
		err := carePlanContributor.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusAccepted, updatedTask.Status)
		})
		t.Run("Check that CareTeam now contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor, *carePlan.CareTeam[0].Reference, participant1, participant2)
		})
	}

	t.Log("Invalid state transition - Accepted -> Completed")
	{
		task.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor.Update("Task/"+*task.Id, task, &updatedTask)
		require.Error(t, err)
	}

	t.Log("Valid state transition - Accepted -> In-progress")
	{
		task.Status = fhir.TaskStatusInProgress
		var updatedTask fhir.Task
		err := carePlanContributor.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusInProgress, updatedTask.Status)
		})
		t.Run("Check that CareTeam still contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor, *carePlan.CareTeam[0].Reference, participant1, participant2)
		})
	}

	t.Log("Complete Task")
	{
		task.Status = fhir.TaskStatusCompleted
		var updatedTask fhir.Task
		err := carePlanContributor.Update("Task/"+*task.Id, task, &updatedTask)
		require.NoError(t, err)
		task = updatedTask

		t.Run("Check Task properties", func(t *testing.T) {
			require.NotNil(t, updatedTask.Id)
			require.Equal(t, fhir.TaskStatusCompleted, updatedTask.Status)
		})
		t.Run("Check that CareTeam now contains the 2 parties", func(t *testing.T) {
			assertCareTeam(t, carePlanContributor, *carePlan.CareTeam[0].Reference, participant1, participant2WithEndDate)
		})
	}
}
func setupIntegrationTest(t *testing.T) *fhirclient.BaseClient {
	fhirBaseURL := setupHAPI(t)
	tokenIntrospectionEndpoint := setupAuthorizationServer(t)

	config := DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	service, err := New(config, nutsPublicURL, orcaPublicURL, tokenIntrospectionEndpoint, "did:web:example.com/careplanservice", nil)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")
	httpClient := httpService.Client()
	httpClient.Transport = auth.AuthenticatedTestRoundTripper(httpService.Client().Transport)

	carePlanContributor := fhirclient.New(carePlanServiceURL, httpClient, nil)
	return carePlanContributor
}

func assertCareTeam(t *testing.T, fhirClient fhirclient.Client, careTeamRef string, expectedMembers ...fhir.CareTeamParticipant) {
	t.Helper()

	var careTeam fhir.CareTeam
	err := fhirClient.Read(careTeamRef, &careTeam)
	require.NoError(t, err)
	require.Lenf(t, careTeam.Participant, len(expectedMembers), "expected %d participants, got %d", len(expectedMembers), len(careTeam.Participant))
	for _, participant := range careTeam.Participant {
		require.NoError(t, coolfhir.ValidateLogicalReference(participant.OnBehalfOf, "Organization", coolfhir.URANamingSystem))
	}

outer:
	for _, expectedMember := range expectedMembers {
		for _, participant := range careTeam.Participant {
			if *participant.OnBehalfOf.Identifier.Value == *expectedMember.OnBehalfOf.Identifier.Value {
				// assert Period
				if expectedMember.Period != nil && expectedMember.Period.Start != nil {
					assert.NotNil(t, participant.Period.Start)
				} else {
					assert.Nil(t, participant.Period.Start)
				}
				if expectedMember.Period != nil && expectedMember.Period.End != nil {
					assert.NotNil(t, participant.Period.End)
				} else {
					assert.Nil(t, participant.Period.End)
				}
				continue outer
			}
		}
		t.Errorf("expected participant not found: %s", *expectedMember.OnBehalfOf.Identifier.Value)
	}
}

// setupAuthorizationServer starts a test OAuth2 authorization server and returns its OAuth2 Token Introspection URL.
func setupAuthorizationServer(t *testing.T) *url.URL {
	authorizationServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestData, _ := io.ReadAll(request.Body)
		if string(requestData) != "token=valid" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		responseData, _ := json.Marshal(map[string]interface{}{
			"active":            true,
			"organization_ura":  "1234",
			"organization_name": "Hospital",
			"organization_city": "CareTown",
		})
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responseData)
	}))
	t.Cleanup(func() {
		authorizationServer.Close()
	})
	u, _ := url.Parse(authorizationServer.URL)
	return u
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
