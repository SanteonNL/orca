//go:generate go run codegen/main.go

package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

type Task map[string]interface{}

func (t Task) ToFHIR() (*fhir.Task, error) {
	taskJSON, _ := json.Marshal(t)
	var result fhir.Task
	if err := json.Unmarshal(taskJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshal Task: %w", err)
	}
	return &result, nil
}

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
		return errors.New("reference cannot be nil")
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

// IdentifierEquals compares two logical identifiers based on their system and value.
// If any of the identifiers is nil or any of the system or value fields is nil, it returns false.
// If the system and value fields of both identifiers are equal, it returns true.
func IdentifierEquals(one *fhir.Identifier, other *fhir.Identifier) bool {
	if one == nil || other == nil {
		return false
	}
	if one.System == nil || other.System == nil {
		return false
	}
	if one.Value == nil || other.Value == nil {
		return false
	}
	return *one.System == *other.System && *one.Value == *other.Value
}
