package careplancontributor

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var notificationCounter = new(atomic.Int32)

func Test_Integration_CPCFHIRProxy(t *testing.T) {
	notificationEndpoint := setupNotificationEndpoint(t)
	carePlanServiceURL, httpService, cpcURL := setupIntegrationTest(t, notificationEndpoint)

	dataHolderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal1, "")
	invalidCareplanTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/999")
	noXSCPHeaderTransport := auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, "")

	cpsDataHolder := fhirclient.New(carePlanServiceURL, &http.Client{Transport: dataHolderTransport}, nil)

	// Create Patient that Task will be related to
	var patient fhir.Patient
	t.Log("Creating Patient")
	{
		patient = fhir.Patient{
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
		}
		err := cpsDataHolder.Create(patient, &patient)
		require.NoError(t, err)
	}

	var carePlan fhir.CarePlan
	var task fhir.Task
	t.Log("Creating Task")
	{
		task = fhir.Task{
			Status:    fhir.TaskStatusRequested,
			Intent:    "order",
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
			Meta: &fhir.Meta{
				Profile: []string{coolfhir.SCPTaskProfile},
			},
			For: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
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
		t.Run("Search for task by ID", func(t *testing.T) {
			var fetchedBundle fhir.Bundle
			err := cpsDataHolder.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
			require.NoError(t, err)
			require.Len(t, fetchedBundle.Entry, 1)
		})
	}

	cpsDataRequester := fhirclient.New(carePlanServiceURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)
	cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal2, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)

	t.Log("Read data from EHR before Task is accepted - Fails")
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

		// Getting patient
		var fetchedPatient fhir.Patient
		err = cpsDataRequester.Read("Patient/"+*patient.Id, &fetchedPatient)
		require.NoError(t, err)
		require.Equal(t, updatedTask.For.Identifier.System, fetchedPatient.Identifier[0].System)
		require.Equal(t, updatedTask.For.Identifier.Value, fetchedPatient.Identifier[0].Value)
		require.Equal(t, *updatedTask.For.Reference, "Patient/"+*fetchedPatient.Id)
	}
	t.Log("Read data from EHR after Task is accepted")
	{
		var fetchedTask fhir.Task

		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.NoError(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.NoError(t, err)
		require.Len(t, fetchedBundle.Entry, 1)
	}
	t.Log("Reading task after accepted - header references non-existent careplan - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: invalidCareplanTransport}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
	t.Log("Reading task after accepted - no xSCP header - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: noXSCPHeaderTransport}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
	t.Log("Reading task after accepted - invalid principal - Fails")
	{
		cpcDataRequester := fhirclient.New(cpcURL, &http.Client{Transport: auth.AuthenticatedTestRoundTripper(httpService.Client().Transport, auth.TestPrincipal3, carePlanServiceURL.String()+"/CarePlan/"+*carePlan.Id)}, nil)
		var fetchedTask fhir.Task
		// Read
		err := cpcDataRequester.Read("Task/"+*task.Id, &fetchedTask)
		require.Error(t, err)
		// Search
		var fetchedBundle fhir.Bundle
		err = cpcDataRequester.Search("Task", url.Values{"_id": {*task.Id}}, &fetchedBundle)
		require.Error(t, err)
		require.Len(t, fetchedBundle.Entry, 0)
	}
}

func setupIntegrationTest(t *testing.T, notificationEndpoint *url.URL) (*url.URL, *httptest.Server, *url.URL) {
	fhirBaseURL := test.SetupHAPI(t)
	config := careplanservice.DefaultConfig()
	config.Enabled = true
	config.FHIR.BaseURL = fhirBaseURL.String()
	config.AllowUnmanagedFHIROperations = true

	fhirClient := fhirclient.New(fhirBaseURL, http.DefaultClient, nil)
	taskengine.LoadTestQuestionnairesAndHealthcareSevices(t, fhirClient)

	activeProfile := profile.TestProfile{
		Principal: auth.TestPrincipal1,
		CSD:       profile.TestCsdDirectory{Endpoint: notificationEndpoint.String()},
	}
	service, err := careplanservice.New(config, activeProfile, orcaPublicURL)
	require.NoError(t, err)

	serverMux := http.NewServeMux()
	httpService := httptest.NewServer(serverMux)
	service.RegisterHandlers(serverMux)

	carePlanServiceURL, _ := url.Parse(httpService.URL + "/cps")
	sessionManager, _ := createTestSession()

	// TODO: Tests using the Zorgplatform service
	cpsProxy := coolfhir.NewProxy("CPS->CPC", fhirBaseURL, "/cpc/fhir", orcaPublicURL, httpService.Client().Transport, true)

	cpcConfig := DefaultConfig()
	cpcConfig.Enabled = true
	cpcConfig.FHIR.BaseURL = fhirBaseURL.String()
	cpcConfig.CarePlanService.URL = carePlanServiceURL.String()
	cpcConfig.HealthDataViewEndpointEnabled = true
	cpc, err := New(cpcConfig, profile.TestProfile{}, orcaPublicURL, sessionManager, cpsProxy)
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
