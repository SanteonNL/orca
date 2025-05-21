package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var _ FHIROperation = &FHIRSanitizeOperationHandler[fhir.HasExtension]{}

// FHIRSanitizeOperationHandler handles sanitization operations on FHIR resources.
// Instead of deleting resources, it updates them to remove most data while preserving
// IDs and relationships, marking them as "entered-in-error" or equivalent.
type FHIRSanitizeOperationHandler[T fhir.HasExtension] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
}

// Handle sanitizes the specified resource.
func (h FHIRSanitizeOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	var resource T
	err := h.fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource)
	if err != nil {
		return nil, err
	}
	resourceID := request.ResourceId

	// Convert to map to sanitize
	var resourceMap map[string]interface{}
	resourceJSON, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource: %w", err)
	}
	if err := json.Unmarshal(resourceJSON, &resourceMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal resource as map: %w", err)
	}

	// Sanitize the resource
	sanitized := sanitizeResource(resourceMap, resourceType)

	// Convert back to JSON and update the resource
	idx := len(tx.Entry)
	tx.Append(sanitized, &fhir.BundleEntryRequest{
		Method: fhir.HTTPVerbPUT,
		Url:    resourceType + "/" + resourceID,
	}, nil)

	// Return an empty bundle entry to indicate success
	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		bundleEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "200 OK",
			},
		}

		// Check if we have a response in the transaction result for the main resource
		if idx < len(txResult.Entry) {
			bundleEntry.Response = txResult.Entry[idx].Response
		}

		return []*fhir.BundleEntry{bundleEntry}, []any{}, nil
	}, nil
}

// sanitizeResource removes most of the data from a resource while preserving IDs and relationships
func sanitizeResource(resource map[string]interface{}, resourceType string) map[string]interface{} {
	// Preserve id and resource type
	id := resource["id"]

	// Start with completely clean resource
	sanitized := map[string]interface{}{
		"resourceType": resourceType,
		"id":           id,
	}

	// Maintain key relationships
	maintainRelationships(resource, sanitized, resourceType)

	return sanitized
}

// maintainRelationships preserves important references in the resource
func maintainRelationships(originalResource, sanitizedResource map[string]interface{}, resourceType string) {
	// Preserve references based on resource type
	switch resourceType {
	case "Task":
		if focus, ok := originalResource["focus"]; ok {
			sanitizedResource["focus"] = focus
		}
		if for_, ok := originalResource["for"]; ok {
			sanitizedResource["for"] = for_
		}
		if owner, ok := originalResource["owner"]; ok {
			sanitizedResource["owner"] = owner
		}
		if requester, ok := originalResource["requester"]; ok {
			sanitizedResource["requester"] = requester
		}
		if basedOn, ok := originalResource["basedOn"]; ok {
			sanitizedResource["basedOn"] = basedOn
		}
	case "CarePlan":
		if subject, ok := originalResource["subject"]; ok {
			sanitizedResource["subject"] = subject
		}
		if careTeam, ok := originalResource["careTeam"]; ok {
			sanitizedResource["careTeam"] = careTeam
		}
		if addresses, ok := originalResource["addresses"]; ok {
			sanitizedResource["addresses"] = addresses
		}
	case "Condition":
		if subject, ok := originalResource["subject"]; ok {
			sanitizedResource["subject"] = subject
		}
	case "ServiceRequest":
		if subject, ok := originalResource["subject"]; ok {
			sanitizedResource["subject"] = subject
		}
		if requester, ok := originalResource["requester"]; ok {
			sanitizedResource["requester"] = requester
		}
	case "QuestionnaireResponse":
		if subject, ok := originalResource["subject"]; ok {
			sanitizedResource["subject"] = subject
		}
		if questionnaire, ok := originalResource["questionnaire"]; ok {
			sanitizedResource["questionnaire"] = questionnaire
		}
	}
}
