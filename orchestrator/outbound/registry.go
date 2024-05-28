package outbound

import (
	"encoding/json"
	"fmt"
	"os"
)

type DIDResolver interface {
	// Resolve resolves the organization identifier to a DID.
	Resolve(organizationIdentifier string) (string, error)
}

func StaticDIDResolverFromJSON(data string) (StaticDIDResolver, error) {
	asMap := make(map[string]string, 0)
	if err := json.Unmarshal([]byte(data), &asMap); err != nil {
		return nil, fmt.Errorf("unable to unmarshal organization identifier->DID mapping: %w", err)
	}
	return asMap, nil
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

var _ DIDResolver = &FileDIDResolver{}

type FileDIDResolver struct {
	File string
}

func (f FileDIDResolver) Resolve(organizationIdentifier string) (string, error) {
	data, err := os.ReadFile(f.File)
	if err != nil {
		return "", fmt.Errorf("unable to read organization identifier->DID mapping file (%s): %w", f.File, err)
	}
	didResolver, err := StaticDIDResolverFromJSON(string(data))
	if err != nil {
		return "", err
	}
	return didResolver.Resolve(organizationIdentifier)
}
