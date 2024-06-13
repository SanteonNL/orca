package addressing

import (
	"fmt"
)

type DIDResolver interface {
	// Resolve resolves the organization identifier to a DID.
	Resolve(organizationIdentifier string) (string, error)
}

var _ DIDResolver = StaticDIDResolver{}

type StaticDIDResolver map[string]string

func (s StaticDIDResolver) Resolve(organizationIdentifier string) (string, error) {
	did, exists := s[organizationIdentifier]
	if !exists {
		return "", fmt.Errorf("unable to find DID for organization identifier: %s", organizationIdentifier)
	}
	return did, nil
}
