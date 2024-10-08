//go:generate mockgen -destination=./interface_mock_test.go -package=csd -source=interface.go
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
	LookupEndpoint(ctx context.Context, owner fhir.Identifier, endpointName string) ([]fhir.Endpoint, error)
}
