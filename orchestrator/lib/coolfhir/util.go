//go:generate go run codegen/main.go

package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"strings"
)

var ErrEntryNotFound = errors.New("entry not found in FHIR Bundle")

func LogicalReference(refType, system, identifier string) *fhir.Reference {
	return &fhir.Reference{
		Type: to.Ptr(refType),
		Identifier: &fhir.Identifier{
			System: &system,
			Value:  &identifier,
		},
	}
}

func FirstIdentifier(identifiers []fhir.Identifier, predicate func(fhir.Identifier) bool) *fhir.Identifier {
	for _, identifier := range identifiers {
		if predicate(identifier) {
			return &identifier
		}
	}
	return nil
}

func FilterNamingSystem(system string) func(fhir.Identifier) bool {
	return func(ident fhir.Identifier) bool {
		return ident.System != nil && *ident.System == system
	}
}

func ValidateLogicalReference(reference *fhir.Reference, expectedType string, expectedSystem string) error {
	if reference == nil {
		return errors.New("not a reference")
	}
	if reference.Type == nil || *reference.Type != expectedType {
		return fmt.Errorf("reference.Type must be %s", expectedType)
	}
	if reference.Identifier == nil || reference.Identifier.System == nil || reference.Identifier.Value == nil {
		return errors.New("reference must contain a logical identifier with a System and Value")
	}
	if *reference.Identifier.System != expectedSystem {
		return fmt.Errorf("reference.Identifier.System must be %s", expectedSystem)
	}
	return nil
}

func IsLogicalReference(reference *fhir.Reference) bool {
	return reference != nil && reference.Type != nil && reference.Identifier != nil && reference.Identifier.System != nil && reference.Identifier.Value != nil
}

// EntryInBundle unmarshals the entry in the bundle that matches the given id into the result.
// If the entry is not found, ErrEntryNotFound is returned.
func EntryInBundle(bundle fhir.Bundle, idOrRef string, result interface{}) error {
	resourceType := ResourceType(result)
	if resourceType == "" {
		return fmt.Errorf("can't infer resouce type from %T", result)
	}
	var id = idOrRef
	if strings.HasPrefix(idOrRef, resourceType+"/") {
		id = strings.TrimPrefix(idOrRef, resourceType+"/")
	}
	for _, entry := range bundle.Entry {
		type TypedResource struct {
			fhir.Resource
			ResourceType string `json:"resourceType"`
		}
		var resource TypedResource
		if json.Unmarshal(entry.Resource, &resource) != nil {
			continue
		}
		if resource.ResourceType == resourceType && resource.Id != nil && *resource.Id == id {
			if err := json.Unmarshal(entry.Resource, result); err != nil {
				return fmt.Errorf("unmarshal Bundle entry (id=%s,target=%T): %w", id, result, err)
			}
			return nil
		}
	}
	return ErrEntryNotFound
}
