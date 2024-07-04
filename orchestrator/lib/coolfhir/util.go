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

type IdentifierPredicate func(identifier fhir.Identifier) bool

func FirstIdentifier(predicate IdentifierPredicate, identifiers ...fhir.Identifier) *fhir.Identifier {
	for _, identifier := range identifiers {
		if predicate(identifier) {
			return &identifier
		}
	}
	return nil
}

func IsNamingSystem(system string) IdentifierPredicate {
	return func(identifier fhir.Identifier) bool {
		return identifier.System != nil && *identifier.System == system
	}
}
