package main

import (
	"e2e-tests/to"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
)

func Test_Main(t *testing.T) {
	rootFHIRBaseURL, _ := url.Parse("http://localhost:9090/fhir")
	rootFHIRClient := fhirclient.New(rootFHIRBaseURL, http.DefaultClient, nil)
	hospitalFHIRBaseURL, _ := url.Parse("http://localhost:9090/fhir/hospital")
	hospitalFHIRClient := fhirclient.New(hospitalFHIRBaseURL, http.DefaultClient, nil)
	clinicFHIRBaseURL, _ := url.Parse("http://localhost:9090/fhir/clinic")
	clinicFHIRClient := fhirclient.New(clinicFHIRBaseURL, http.DefaultClient, nil)

	println("Creating HAPI FHIR tenants...")
	tenants := []string{"clinic", "hospital"}
	for i, tenantName := range tenants {
		err := createTenant(tenantName, i+1, rootFHIRClient)
		require.NoError(t, err, fmt.Sprintf("Failed to create tenant: %s", tenantName))
	}
	println("Loading test data...")
	patient, serviceRequest, err := loadTestData(hospitalFHIRClient)
	require.NoError(t, err)

	existingTaskIDs, err := listTaskIDs(clinicFHIRClient)
	require.NoError(t, err)

	// Demo AppLaunch
	println("Demo AppLaunch...")
	cookieJar, _ := cookiejar.New(nil)
	userAgent := &http.Client{
		Jar: cookieJar,
	}
	query := url.Values{}
	query.Add("iss", "http://fhirstore:8080/fhir/hospital")
	query.Add("patient", "Patient/"+*patient.Id)
	query.Add("serviceRequest", "ServiceRequest/"+*serviceRequest.Id)
	query.Add("practitioner", "the-doctor")
	httpResponse, err := userAgent.Get("http://localhost:8080/hospital/orca/demo-app-launch?" + query.Encode())
	testHTTPResponse(err, httpResponse, http.StatusOK)
	//
	// Click "confirm"
	//
	println("Clicking 'confirm'...")
	go func() {
		httpResponse, err = userAgent.PostForm("http://localhost:8080/hospital/orca/contrib/confirm", nil)
		testHTTPResponse(err, httpResponse, http.StatusOK)
	}()
	//
	// Wait for the Task to arrive
	//
	println("Waiting for the new Task to arrive at the CarePlanService...")
	var task *fhir.Task
	waitFor(t, 10*time.Second, func() (bool, error) {
		task, err = findNewTask(clinicFHIRClient, existingTaskIDs)
		return task != nil, err
	}, "Task arrived at the CarePlanService")
	require.Equal(t, fhir.TaskStatusRequested, task.Status, "unexpected Task status")
	//
	// Set accepted
	//
	println("Setting Task status to 'accepted'...")
	task.Status = fhir.TaskStatusAccepted
	require.NoError(t, clinicFHIRClient.Update("Task/"+*task.Id, task, &task)) // TODO: Change this to the CPS client
	//
	// Wait for the Task to updated
	//
	println("Waiting for the Task to be updated...")
	var updatedTask map[string]interface{}
	waitFor(t, 10*time.Second, func() (bool, error) {
		if err := clinicFHIRClient.Read("Task/"+*task.Id, &updatedTask); err != nil {
			return false, err
		}
		contained, ok := updatedTask["contained"].([]interface{})
		return ok && len(contained) >= 2, nil
	}, "Task with ServiceRequest/Patient")
	containedResources := updatedTask["contained"].([]interface{})
	require.Equal(t, containedResources[0].(map[string]interface{})["resourceType"], "ServiceRequest")
	require.Equal(t, containedResources[1].(map[string]interface{})["resourceType"], "Patient")
	println("Test succeeded!")
}

func createTenant(name string, id int, fhirClient *fhirclient.BaseClient) error {
	println("Creating tenant: " + name)
	var tenant fhir.Parameters
	tenant.Parameter = []fhir.ParametersParameter{
		{
			Name:         "id",
			ValueInteger: to.Ptr(id),
		},
		{
			Name:        "name",
			ValueString: to.Ptr(name),
		},
	}
	err := fhirClient.Create(tenant, &tenant, fhirclient.AtPath("DEFAULT/$partition-management-create-partition"))
	if err != nil && strings.Contains(err.Error(), "status=400") {
		// assume it's OK (maybe it already exists)
		return nil
	}
	return err
}

func testHTTPResponse(err error, httpResponse *http.Response, expectedStatus int) {
	if err != nil {
		panic(err)
	}
	if httpResponse.StatusCode != expectedStatus {
		responseData, _ := io.ReadAll(httpResponse.Body)
		println("Response data:\n----------------\n", strings.TrimSpace(string(responseData)), "\n----------------")
		panic(fmt.Sprintf("unexpected status code (status=%s, expected=%d, url=%s)", httpResponse.Status, expectedStatus, httpResponse.Request.URL))
	}
}

func loadTestData(fhirClient *fhirclient.BaseClient) (*fhir.Patient, *fhir.ServiceRequest, error) {
	// Patient
	var patient fhir.Patient
	if err := loadResource("data/Patient-minimal-enrollment-Patient.json", &patient); err != nil {
		return nil, nil, err
	}
	if err := createResource(fhirClient, &patient); err != nil {
		return nil, nil, err
	}
	// Condition
	var condition fhir.Condition
	if err := loadResource("data/Condition-minimal-enrollment-Condition.json", &condition); err != nil {
		return nil, nil, err
	}
	condition.Subject = fhir.Reference{
		Reference: to.Ptr("Patient/" + *patient.Id),
		Type:      to.Ptr("Patient"),
	}
	if err := createResource(fhirClient, &condition); err != nil {
		return nil, nil, err
	}
	// ServiceRequest
	var serviceRequest fhir.ServiceRequest
	if err := loadResource("data/ServiceRequest-minimal-enrollment-ServiceRequest.json", &serviceRequest); err != nil {
		return nil, nil, err
	}
	serviceRequest.Subject = fhir.Reference{
		Reference: to.Ptr("Patient/" + *patient.Id),
		Type:      to.Ptr("Patient"),
	}
	serviceRequest.ReasonReference = []fhir.Reference{
		{
			Reference: to.Ptr("Condition/" + *condition.Id),
			Type:      to.Ptr("Condition"),
		},
	}
	if err := createResource(fhirClient, &serviceRequest); err != nil {
		return nil, nil, err
	}
	return &patient, &serviceRequest, nil
}

func findNewTask(fhirClient *fhirclient.BaseClient, existingTaskIDs []string) (*fhir.Task, error) {
	var taskBundle fhir.Bundle
	if err := fhirClient.Read("Task", &taskBundle); err != nil {
		return nil, err
	}
outer:
	for _, entry := range taskBundle.Entry {
		var task fhir.Task
		if err := json.Unmarshal(entry.Resource, &task); err != nil {
			return nil, err
		}
		for _, existingTaskID := range existingTaskIDs {
			if *task.Id == existingTaskID {
				continue outer
			}
		}
		return &task, nil
	}
	return nil, nil
}

func listTaskIDs(fhirClient *fhirclient.BaseClient) ([]string, error) {
	var taskBundle fhir.Bundle
	if err := fhirClient.Read("Task", &taskBundle); err != nil {
		return nil, err
	}
	var taskIDs []string
	for _, entry := range taskBundle.Entry {
		var task fhir.Task
		if err := json.Unmarshal(entry.Resource, &task); err != nil {
			return nil, err
		}
		taskIDs = append(taskIDs, *task.Id)
	}
	return taskIDs, nil

}

func createResource(fhirClient *fhirclient.BaseClient, resource interface{}) error {
	return fhirClient.Create(resource, resource)
}

func loadResource(fileName string, resource interface{}) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, resource); err != nil {
		return err
	}
	return nil
}

func waitFor(t *testing.T, timeOut time.Duration, predicate func() (bool, error), msg string) {
	startTime := time.Now()
	for {
		if time.Since(startTime) > timeOut {
			require.Fail(t, "Time-out while waiting for condition: "+msg)
		}
		ok, err := predicate()
		if err != nil {
			require.Fail(t, "Error while waiting for condition: "+err.Error())
		}
		if ok {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}
