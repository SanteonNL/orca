//go:generate mockgen -destination=./test.go -package=csd -source=interface.go
package csd

import (
	"context"
	"errors"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var ErrEntryNotFound = errors.New("CSD does not contain the specified entry")

// Directory defines the primary interface to discovery Care Services in the CSD.
type Directory interface {
	// LookupEndpoint searches for endpoints with the given name of the given owner.
	// If the owner is nil, it will search for all endpoints in the CSD.
	LookupEndpoint(ctx context.Context, owner *fhir.Identifier, endpointName string) ([]fhir.Endpoint, error)
	// LookupEntity searches for an entity with the given identifier, returning it as reference.
	// The reference then might contain more information on the entity, like a human-readable display name.
	// If the entity is not found, it returns ErrEntryNotFound.
	LookupEntity(ctx context.Context, identifier fhir.Identifier) (*fhir.Reference, error)
}
