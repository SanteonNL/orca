package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Target organization identifier - hardcoded as this is a one-off utility
var targetOrganization = fhir.Identifier{
	System: stringPtr("https://example.org/organization-ids"),
	Value:  stringPtr("target-org-001"),
}

// Reference to the target organization
var targetOrgReference = fhir.Reference{
	Identifier: &targetOrganization,
	Display:    stringPtr("Target Organization"),
}

// Global storage for CURL commands
var curlCommands []string

// main modifies Task, ServiceRequest, and CarePlan resources based on specified rules:
// - Task: If Owner and Requester are equal, change Owner to target organization
// - ServiceRequest: Set performer to target organization if not already set
// - CarePlan: Check CareTeam contained resource, modify duplicate members
// Set DRY_RUN=true to generate CURL commands without executing updates
// Remember to set the FHIR_BASEURL and FHIR_BEARER_TOKEN environment variables before running this script.
func main() {
	// Environment variables
	var fhirBaseURL = os.Getenv("FHIR_BASEURL")
	if fhirBaseURL == "" {
		panic("FHIR_BASEURL environment variable is not set")
	}

	var token = os.Getenv("FHIR_BEARER_TOKEN")
	if token == "" {
		panic("FHIR_BEARER_TOKEN environment variable is not set")
	}

	// Parse DRY_RUN environment variable (defaults to false)
	dryRun := false
	if dryRunStr := os.Getenv("DRY_RUN"); dryRunStr != "" {
		var err error
		dryRun, err = strconv.ParseBool(dryRunStr)
		if err != nil {
			panic(fmt.Sprintf("Invalid DRY_RUN value: %s. Must be true or false", dryRunStr))
		}
	}

	fmt.Printf("Running in mode: %s\n", map[bool]string{true: "DRY_RUN", false: "EXECUTE"}[dryRun])
	fmt.Printf("Target Organization: %s (system: %s)\n",
		*targetOrganization.Value, *targetOrganization.System)

	parsedBaseURL, err := url.Parse(fhirBaseURL)
	if err != nil {
		panic(err)
	}

	client := fhirclient.New(parsedBaseURL, &staticTokenRequestDoer{token}, nil)

	// Initialize CURL commands storage
	curlCommands = make([]string, 0)

	// Process each resource type with dedicated methods
	if err := processTasks(client, dryRun); err != nil {
		panic(fmt.Sprintf("Error processing Task resources: %v", err))
	}

	if err := processServiceRequests(client, dryRun); err != nil {
		panic(fmt.Sprintf("Error processing ServiceRequest resources: %v", err))
	}

	if err := processCarePlans(client, dryRun); err != nil {
		panic(fmt.Sprintf("Error processing CarePlan resources: %v", err))
	}

	fmt.Println("Processing complete!")

	// Print all CURL commands at the end if in dry-run mode
	if dryRun && len(curlCommands) > 0 {
		fmt.Printf("\n=== CURL COMMANDS ===\n")
		for i, cmd := range curlCommands {
			fmt.Printf("\n# Command %d\n%s\n", i+1, cmd)
		}
		fmt.Printf("\n=== END CURL COMMANDS ===\n")
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

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// processTasks searches for and processes all Task resources
// Rule: If Owner and Requester are equal, change Owner to target organization
func processTasks(client fhirclient.Client, dryRun bool) error {
	fmt.Printf("Processing Task resources...\n")

	var searchResults fhir.Bundle
	if err := client.Search("Task", nil, &searchResults); err != nil {
		return fmt.Errorf("failed to search for Task resources: %w", err)
	}

	fmt.Printf("Found %d Task resources to process\n", len(searchResults.Entry))
	modifiedCount := 0

	for _, entry := range searchResults.Entry {
		var task fhir.Task
		if err := json.Unmarshal(entry.Resource, &task); err != nil {
			fmt.Printf("Warning: failed to unmarshal Task resource, skipping: %v\n", err)
			continue
		}

		if task.Id == nil {
			fmt.Printf("Warning: Task resource missing ID, skipping\n")
			continue
		}

		// Check if Owner and Requester are equal
		if task.Owner == nil || task.Requester == nil {
			continue
		}

		if referencesEqual(task.Owner, task.Requester) {
			// Update Owner to target organization
			task.Owner = &targetOrgReference

			if dryRun {
				if err := storeCurlCommand("Task", *task.Id, &task); err != nil {
					fmt.Printf("Error generating CURL command for Task/%s: %v\n", *task.Id, err)
				} else {
					modifiedCount++
				}
			} else {
				if err := updateResource(client, "Task", *task.Id, &task); err != nil {
					fmt.Printf("Error updating Task/%s: %v\n", *task.Id, err)
				} else {
					modifiedCount++
				}
			}
		}
	}

	fmt.Printf("Modified %d Task resources\n", modifiedCount)
	return nil
}

// processServiceRequests searches for and processes all ServiceRequest resources
// Rule: Set performer to target organization if not already set
func processServiceRequests(client fhirclient.Client, dryRun bool) error {
	fmt.Printf("Processing ServiceRequest resources...\n")

	var searchResults fhir.Bundle
	if err := client.Search("ServiceRequest", nil, &searchResults); err != nil {
		return fmt.Errorf("failed to search for ServiceRequest resources: %w", err)
	}

	fmt.Printf("Found %d ServiceRequest resources to process\n", len(searchResults.Entry))
	modifiedCount := 0

	for _, entry := range searchResults.Entry {
		var serviceRequest fhir.ServiceRequest
		if err := json.Unmarshal(entry.Resource, &serviceRequest); err != nil {
			fmt.Printf("Warning: failed to unmarshal ServiceRequest resource, skipping: %v\n", err)
			continue
		}

		if serviceRequest.Id == nil {
			fmt.Printf("Warning: ServiceRequest resource missing ID, skipping\n")
			continue
		}

		// Check if performer needs to be set to target organization
		needsUpdate := false

		if serviceRequest.Performer == nil || len(serviceRequest.Performer) == 0 {
			serviceRequest.Performer = []fhir.Reference{targetOrgReference}
			needsUpdate = true
		} else {
			// Check if target organization is already in performers
			hasTargetOrg := false
			for _, performer := range serviceRequest.Performer {
				if referencesEqual(&performer, &targetOrgReference) {
					hasTargetOrg = true
					break
				}
			}

			if !hasTargetOrg {
				serviceRequest.Performer = append(serviceRequest.Performer, targetOrgReference)
				needsUpdate = true
			}
		}

		if needsUpdate {
			if dryRun {
				if err := storeCurlCommand("ServiceRequest", *serviceRequest.Id, &serviceRequest); err != nil {
					fmt.Printf("Error generating CURL command for ServiceRequest/%s: %v\n", *serviceRequest.Id, err)
				} else {
					modifiedCount++
				}
			} else {
				if err := updateResource(client, "ServiceRequest", *serviceRequest.Id, &serviceRequest); err != nil {
					fmt.Printf("Error updating ServiceRequest/%s: %v\n", *serviceRequest.Id, err)
				} else {
					modifiedCount++
				}
			}
		}
	}

	fmt.Printf("Modified %d ServiceRequest resources\n", modifiedCount)
	return nil
}

// processCarePlans searches for and processes all CarePlan resources
// Rule: Check CareTeam contained resource, if 2 members are the same, change one to target organization
func processCarePlans(client fhirclient.Client, dryRun bool) error {
	fmt.Printf("Processing CarePlan resources...\n")

	var searchResults fhir.Bundle
	if err := client.Search("CarePlan", nil, &searchResults); err != nil {
		return fmt.Errorf("failed to search for CarePlan resources: %w", err)
	}

	fmt.Printf("Found %d CarePlan resources to process\n", len(searchResults.Entry))
	modifiedCount := 0

	for _, entry := range searchResults.Entry {
		var carePlan fhir.CarePlan
		if err := json.Unmarshal(entry.Resource, &carePlan); err != nil {
			fmt.Printf("Warning: failed to unmarshal CarePlan resource, skipping: %v\n", err)
			continue
		}

		if carePlan.Id == nil {
			fmt.Printf("Warning: CarePlan resource missing ID, skipping\n")
			continue
		}

		// Extract CareTeam using coolfhir utility
		careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
		if err != nil {
			continue
		}

		// Check if CareTeam has exactly 2 members and they are the same
		modified := false
		if careTeam.Participant != nil && len(careTeam.Participant) == 2 {
			member1 := careTeam.Participant[0].Member
			member2 := careTeam.Participant[1].Member

			if member1 != nil && member2 != nil && referencesEqual(member1, member2) {
				// Change the second member to target organization
				careTeam.Participant[1].Member = &targetOrgReference
				modified = true
			}
		}

		if modified {
			// Update the contained CareTeam using coolfhir utility
			if len(carePlan.CareTeam) > 0 {
				contained, err := coolfhir.UpdateContainedResource(carePlan.Contained, &carePlan.CareTeam[0], careTeam)
				if err != nil {
					fmt.Printf("Error updating contained CareTeam for CarePlan/%s: %v\n", *carePlan.Id, err)
					continue
				}
				carePlan.Contained = contained
			}

			if dryRun {
				if err := storeCurlCommand("CarePlan", *carePlan.Id, &carePlan); err != nil {
					fmt.Printf("Error generating CURL command for CarePlan/%s: %v\n", *carePlan.Id, err)
				} else {
					modifiedCount++
				}
			} else {
				if err := updateResource(client, "CarePlan", *carePlan.Id, &carePlan); err != nil {
					fmt.Printf("Error updating CarePlan/%s: %v\n", *carePlan.Id, err)
				} else {
					modifiedCount++
				}
			}
		}
	}

	fmt.Printf("Modified %d CarePlan resources\n", modifiedCount)
	return nil
}

// referencesEqual compares two FHIR references using existing coolfhir utilities
func referencesEqual(ref1, ref2 *fhir.Reference) bool {
	if ref1 == nil || ref2 == nil {
		return ref1 == ref2
	}

	// Use coolfhir utilities for proper comparison
	// First try logical reference comparison (by identifier)
	if coolfhir.LogicalReferenceEquals(*ref1, *ref2) {
		return true
	}

	// Then try reference value comparison (by reference string and type)
	return coolfhir.ReferenceValueEquals(*ref1, *ref2)
}

// storeCurlCommand stores a CURL command for later printing (dry-run mode)
func storeCurlCommand(resourceType, resourceId string, resource interface{}) error {
	resourceJson, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resource for CURL command: %w", err)
	}

	curlCmd := fmt.Sprintf("curl -X PUT \\\n  \"${FHIR_BASEURL}/%s/%s\" \\\n  -H \"Authorization: Bearer ${FHIR_BEARER_TOKEN}\" \\\n  -H \"Content-Type: application/fhir+json\" \\\n  -d '%s'",
		resourceType, resourceId, string(resourceJson))

	curlCommands = append(curlCommands, curlCmd)
	return nil
}

// updateResource performs the actual FHIR update (execute mode)
func updateResource(client fhirclient.Client, resourceType, resourceId string, resource interface{}) error {
	// Convert the resource to JSON for the update
	//resourceJson, err := json.Marshal(resource)
	//if err != nil {
	//	return fmt.Errorf("failed to marshal resource for update: %w", err)
	//}
	//
	//// Create the update path
	//updatePath := fmt.Sprintf("%s/%s", resourceType, resourceId)
	//
	//// Use the FHIR client's Update method
	//if err := client.Update(updatePath, resourceJson, nil); err != nil {
	//	return fmt.Errorf("failed to update FHIR resource %s: %w", updatePath, err)
	//}

	return nil
}
