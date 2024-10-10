package careplancontributor

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

var notificationCounter = new(atomic.Int32)

func Test_Integration_CPCFHIRProxy(t *testing.T) {
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanServiceURL, httpService, cpcURL := setupIntegrationTest(t, notificationEndpoint)

	dataHolderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal1, "")
	dataRequesterTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/1")
	invalidCareplanTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/2")
	noXSCPHeaderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, "")
	// Test principal is not part of the care team
	invalidTestPrincipalTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal3, carePlanServiceURL.String()+"/CarePlan/1")

	cpsDataHolder := fhirclient.New(carePlanServiceURL, &http.Client{Transport: dataHolderTransport}, nil)
	cpsDataRequester := fhirclient.New(carePlanServiceURL, &http.Client{Transport: dataRequesterTransport}, nil)

	var carePlan fhir.CarePlan
	var task fhir.Task
	t.Log("Creating Task")
	{
		task = fhir.Task{
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}

		err := cpsDataHolder.Create(task, &task)
		require.NoError(t, err)
		err = cpsDataHolder.Read(*task.BasedOn[0].Reference, &carePlan)
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
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: dataRequesterTransport}, nil)
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
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: dataRequesterTransport}, nil)
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.NoError(t, err)
	}
	t.Log("Reading task after accepted - header references non-existent careplan - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: invalidCareplanTransport}, nil)
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Reading task after accepted - no xSCP header - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: noXSCPHeaderTransport}, nil)
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
	t.Log("Reading task after accepted - invalid principal - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: invalidTestPrincipalTransport}, nil)
		var fetchedTask fhir.Task
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
	}
}

func setupIntegrationTest(t *testing.T, notificationEndpoint *url.URL) (*url.URL, *httptest.Server, *url.URL) {
	fhirBaseURL := test.SetupHAPI(t)
	config := careplanservice.DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	service, err := careplanservice.New(config, profile.TestProfile{
		TestCsdDirectory: profile.TestCsdDirectory{Endpoint: notificationEndpoint.String()},
	}, orcaPublicURL)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")

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
	cpcURL, _ := url.Parse(cpcHttpService.URL + "/cpc/fhir")

	// Return the URLs and httpService of the cpsDataRequester, cpcDataRequester, cpsDataHolder
	// these will be used to construct clients with both the correct and incorrect auth for positive and negative testing
	return carePlanServiceURL, httpService, cpcURL
}

func setupNotificationEndpoint(t *testing.T) *url.URL {
	notificationEndpoint := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		notificationCounter.Add(1)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(func() {
		notificationEndpoint.Close()
	})
	u, _ := url.Parse(notificationEndpoint.URL)
	return u
}
