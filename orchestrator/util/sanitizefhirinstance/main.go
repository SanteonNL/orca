package main

import (
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"log"
	"net/http"
	"net/url"
	"os"
)

// main sanitizes all resources of the specified types from the FHIR server.
// It performs a search for each resource type and sanitizes all resources of that type.
// Remember to set the FHIR_BASEURL and FHIR_BEARER_TOKEN environment variables before running this script.
func main() {
	resourceTypesToSanitize := []string{
		"Patient",
		"CarePlan",
		"Task",
		"Condition",
		"ServiceRequest",
		"QuestionnaireResponse",
	}

	var fhirBaseURL = os.Getenv("FHIR_BASEURL")
	if fhirBaseURL == "" {
		panic("FHIR_BASEURL environment variable is not set")
	}
	var token = os.Getenv("FHIR_BEARER_TOKEN")
	if token == "" {
		panic("FHIR_BEARER_TOKEN environment variable is not set")
	}

	externalEndpoint := fhirBaseURL + "/orca/cpc/external/fhir"

	parsedBaseURL, err := url.Parse(externalEndpoint)
	if err != nil {
		panic(err)
	}
	client := fhirclient.New(parsedBaseURL, &staticTokenRequestDoer{token, fhirBaseURL}, nil)
	for _, resType := range resourceTypesToSanitize {
		log.Println("Sanitizing resources of type " + resType)
		if err := sanitizeResourcesOfType(client, resType); err != nil {
			panic(err)
		}
	}

	log.Println("Resource sanitization completed successfully")
}

var _ fhirclient.HttpRequestDoer = &staticTokenRequestDoer{}

type staticTokenRequestDoer struct {
	token       string
	fhirBaseURL string
}

func (s staticTokenRequestDoer) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("X-Scp-Fhir-Url", s.fhirBaseURL+"/orca/cps")
	return http.DefaultClient.Do(req)
}

type BaseResource struct {
	Id string `json:"id"`
}

func sanitizeResourcesOfType(client fhirclient.Client, resourceType string) error {
	do := true
	for do {
		var searchResults fhir.Bundle
		if err := client.Search(resourceType, nil, &searchResults); err != nil {
			return err
		}
		// Keep repeating until no more results are returned
		//do = len(searchResults.Entry) > 0
		for _, entry := range searchResults.Entry {
			var resource BaseResource
			if err := json.Unmarshal(entry.Resource, &resource); err != nil {
				return fmt.Errorf("failed to unmarshal resource: %w", err)
			}
			log.Println("Sanitizing resource " + resourceType + "/" + resource.Id)
			if err := sanitizeResource(client, resourceType, resource.Id); err != nil {
				log.Println("failed to sanitize resource: %w", err)
			}
		}
		do = false
	}
	return nil
}

func sanitizeResource(client fhirclient.Client, resourceType string, resourceId string) error {
	// Create an empty resource of the appropriate type to send in the PUT request
	var requestBody interface{}

	// Initialize the appropriate resource type
	//switch resourceType {
	//case "Patient":
	//	requestBody = fhir.Patient{}
	//case "CarePlan":
	//	requestBody = fhir.CarePlan{}
	//case "Task":
	//	requestBody = fhir.Task{}
	//case "Condition":
	//	requestBody = fhir.Condition{}
	//case "ServiceRequest":
	//	requestBody = fhir.ServiceRequest{}
	//case "QuestionnaireResponse":
	//	requestBody = fhir.QuestionnaireResponse{}
	//case "Questionnaire":
	//	requestBody = fhir.Questionnaire{}
	//case "Organization":
	//	requestBody = fhir.Organization{}
	//default:
	//	// For unknown types, use a generic map
	//	requestBody = map[string]interface{}{}
	//}
	requestBody = map[string]interface{}{}

	// PUT to the sanitize endpoint
	sanitizeEndpoint := resourceType + "/" + resourceId + "/$sanitize"
	if err := client.Update(sanitizeEndpoint, requestBody, nil); err != nil {
		return err
	}

	println("Sanitized " + resourceType + "/" + resourceId)
	return nil
}
