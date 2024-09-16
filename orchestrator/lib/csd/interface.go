package csd

import (
	"context"
	"errors"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

var ErrEntryNotFound = errors.New("CSD does not contain the specified entry")

// Directory defines the primary interface to discovery Care Services in the CSD.
type Directory interface {
	// LookupEndpoint searches for endpoints with the given name for the given service and owner.
	LookupEndpoint(ctx context.Context, owner fhir.Identifier, service string, endpointName string) ([]fhir.Endpoint, error)
}
