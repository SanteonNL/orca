package main

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

//go:embed resources.txt
var resourcesTxt []byte

// main runs a program that reads FHIR resource references (e.g. "Patient/123") from a file (resources.txt),
// queries the FHIR server for each resource, checks if it is deleted (HTTP 410 Gone),
// and if so, restores it by finding the latest non-DELETED version in the history and updating the resource to that version.
func main() {
	fhirBaseURL, _ := url.Parse("")
	token := ""
	fhirClient := fhirclient.New(fhirBaseURL, &staticTokenRequestDoer{token}, nil)

	resourcesLines := string(resourcesTxt)
	// Read each line
	lines := strings.Split(resourcesLines, "\n")
	const maxResourcesToProcess = 1
	processed := 0
	for _, resourceRef := range lines {
		if processed >= maxResourcesToProcess {
			println("Reached maximum number of rolled back resources:", maxResourcesToProcess)
			break
		}

		// For each resource: query the history, check if it needs restoring (fetch it), find the latest non-DELETED version, and restore than one using an update operation.
		resourceRef = strings.TrimSpace(resourceRef)
		if resourceRef == "" {
			continue // Skip empty lines
		}

		resource := make(map[string]any)
		var responseStatus int
		if err := fhirClient.Read(resourceRef, &resource, fhirclient.ResponseStatusCode(&responseStatus)); err != nil {
			if responseStatus != http.StatusGone {
				panic("Failed to read resource " + resourceRef + ": " + err.Error())
			}
		} else {
			if responseStatus != http.StatusGone {
				// No need to resource this resource
				println("Resource", resourceRef, "is not deleted, skipping.")
				continue
			}
		}

		var historyBundle fhir.Bundle
		if err := fhirClient.Read(resourceRef+"/_history", &historyBundle); err != nil {
			panic(err)
		}
		if historyBundle.Type != fhir.BundleTypeHistory {
			panic("Expected a history bundle, got: " + historyBundle.Type.String())
		}
		// Find the latest non-DELETED version
		var latestVersion *fhir.BundleEntry
		var latestVersionId int
		for _, entry := range historyBundle.Entry {
			if entry.Resource == nil {
				continue // Skip deleted resources
			}
			if entry.Request.Method == fhir.HTTPVerbDELETE {
				continue // Skip deleted resources
			}
			if err := json.Unmarshal(entry.Resource, &resource); err != nil {
				panic("Failed to unmarshal resource: " + err.Error())
			}
			versionId, err := strconv.Atoi(resource["meta"].(map[string]any)["versionId"].(string))
			if err != nil {
				panic("Failed to parse versionId: " + err.Error())
			}
			if latestVersionId == 0 || versionId > latestVersionId {
				latestVersionId = versionId
				latestVersion = &entry
			}
		}
		if latestVersion == nil {
			panic("No non-DELETED version found for resource: " + resourceRef)
		}
		println("Rolling back resource", resourceRef, "to version", latestVersionId)

		if err := fhirClient.Update(resourceRef, latestVersion.Resource, &resource); err != nil {
			panic("Failed to update resource " + resourceRef + ": " + err.Error())
		} else {
			println("Rolled back resource", resourceRef, "to version", latestVersionId)
		}

		processed++
	}
	println("Rolled back", processed, "resources.")
}

type staticTokenRequestDoer struct {
	token string
}

func (s staticTokenRequestDoer) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	return http.DefaultClient.Do(req)
}
