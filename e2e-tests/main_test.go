package main

import (
	"e2e-tests/to"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
)

func Test_Main(t *testing.T) {
	fhirBaseURL, _ := url.Parse("http://localhost:9090/fhir")
	fhirClient := fhirclient.New(fhirBaseURL, http.DefaultClient)

	println("Loading test data...")
	patient, serviceRequest, err := loadTestData(fhirClient)
	require.NoError(t, err)

	existingTaskIDs, err := listTaskIDs(fhirClient)
	require.NoError(t, err)

	// Demo AppLaunch
	println("Demo AppLaunch...")
	cookieJar, _ := cookiejar.New(nil)
	userAgent := &http.Client{
		Jar: cookieJar,
	}
	query := url.Values{}
	query.Add("iss", "http://fhirstore:8080/fhir")
	query.Add("patient", "Patient/"+*patient.Id)
	query.Add("serviceRequest", "ServiceRequest/"+*serviceRequest.Id)
	query.Add("practitioner", "the-doctor")
	httpResponse, err := userAgent.Get("http://localhost:8080/hospital/orca/demo-app-launch?" + query.Encode())
	testHTTPResponse(err, httpResponse, http.StatusOK)
	// Click "confirm"
	println("Clicking 'confirm'...")
	httpResponse, err = userAgent.PostForm(httpResponse.Request.URL.JoinPath("confirm").String(), nil)
	testHTTPResponse(err, httpResponse, http.StatusOK)
	// Check that the Task arrived at the CarePlanService
	println("Checking that the Task arrived at the CarePlanService...")
	task, err := findNewTask(fhirClient, existingTaskIDs)
	require.NoError(t, err)
	require.Equal(t, fhir.TaskStatusRequested, task.Status, "unexpected Task status")
	// Set accepted
	println("Setting Task status to 'accepted'...")
	task.Status = fhir.TaskStatusAccepted
	require.NoError(t, fhirClient.Update("Task/"+*task.Id, task, &task))

	println("Test succeeded!")
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
	// Requester
	var requester fhir.Organization
	if err := loadResource("data/Organization-minimal-enrollment-Organization-Requester.json", &requester); err != nil {
		return nil, nil, err
	}
	if err := createResource(fhirClient, &requester); err != nil {
		return nil, nil, err
	}
	// Performer
	var performer fhir.Organization
	if err := loadResource("data/Organization-minimal-enrollment-Organization-Performer.json", &performer); err != nil {
		return nil, nil, err
	}
	if err := createResource(fhirClient, &performer); err != nil {
		return nil, nil, err
	}
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
	serviceRequest.Performer = []fhir.Reference{
		{
			Reference: to.Ptr("Organization/" + *performer.Id),
			Type:      to.Ptr("Organization"),
		},
	}
	serviceRequest.Requester = &fhir.Reference{
		Reference: to.Ptr("Organization/" + *requester.Id),
		Type:      to.Ptr("Organization"),
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
	return nil, errors.New("no new Task found")
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
