//go:generate go run codegen/main.go

package coolfhir

import (
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func LogicalReference(refType, system, identifier string) *fhir.Reference {
	return &fhir.Reference{
		Type: to.Ptr(refType),
		Identifier: &fhir.Identifier{
			System: &system,
			Value:  &identifier,
		},
	}
}

// LogicalReferenceEquals checks if two references are contain the same logical identifier, given their system and value.
// It does not compare identifier type.
func LogicalReferenceEquals(ref, other fhir.Reference) bool {
	return ref.Identifier != nil && other.Identifier != nil &&
		ref.Identifier.System != nil && other.Identifier.System != nil && *ref.Identifier.System == *other.Identifier.System &&
		ref.Identifier.Value != nil && other.Identifier.Value != nil && *ref.Identifier.Value == *other.Identifier.Value
}

func FirstIdentifier(identifiers []fhir.Identifier, predicate func(fhir.Identifier) bool) *fhir.Identifier {
	for _, identifier := range identifiers {
		if predicate(identifier) {
			return &identifier
		}
	}
	return nil
}

func IsNamingSystem(system string) func(fhir.Identifier) bool {
	return func(ident fhir.Identifier) bool {
		return ident.System != nil && *ident.System == system
	}
}
