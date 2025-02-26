package main

import (
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

// main deletes all resources of the specified types from the FHIR server.
// It performs a search for each resource type and deletes all resources of that type.
// Remember to set the token and baseURL variables before running this script.
func main() {
	resourceTypesToDelete := []string{
		"Patient",
		"CarePlan",
		"CareTeam",
		"Task",
		"Condition",
		"ServiceRequest",
		"QuestionnaireResponse",
		"Organization",
	}

	var token = ""
	var baseURL = ""

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		panic(err)
	}
	client := fhirclient.New(parsedBaseURL, &staticTokenRequestDoer{token}, nil)
	for _, resType := range resourceTypesToDelete {
		if err := deleteResourcesOfType(client, resType); err != nil {
			panic(err)
		}
	}
}

var _ fhirclient.HttpRequestDoer = &staticTokenRequestDoer{}

type staticTokenRequestDoer struct {
	token string
}

func (s staticTokenRequestDoer) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	return http.DefaultClient.Do(req)
}

func deleteResourcesOfType(client fhirclient.Client, resourceType string) error {
	do := true
	for do {
		var searchResults fhir.Bundle
		if err := client.Search(resourceType, nil, &searchResults); err != nil {
			return err
		}
		// Keep repeating until no more results are returned
		do = len(searchResults.Entry) > 0
		for _, entry := range searchResults.Entry {
			var resource test.BaseResource
			if err := json.Unmarshal(entry.Resource, &resource); err != nil {
				return fmt.Errorf("failed to unmarshal resource: %w", err)
			}
			if err := deleteResource(client, resourceType, resource.Id); err != nil {
				return fmt.Errorf("failed to delete resource: %w", err)
			}
		}
	}
	return nil
}

func deleteResource(client fhirclient.Client, resourceType string, resourceId string) error {
	if err := client.Delete(resourceType + "/" + resourceId); err != nil {
		return err
	}
	println("Deleted " + resourceType + "/" + resourceId)
	return nil
}
