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

var _ DIDResolver = &FileDIDResolver{}

type FileDIDResolver struct {
	File string
}

func (f FileDIDResolver) Resolve(organizationIdentifier string) (string, error) {
	data, err := os.ReadFile(f.File)
	if err != nil {
		return "", fmt.Errorf("unable to read organization identifier->DID mapping file (%s): %w", f.File, err)
	}
	asMap := make(map[string]string, 0)
	if err := json.Unmarshal(data, &asMap); err != nil {
		return "", fmt.Errorf("unable to unmarshal organization identifier->DID mapping file (%s): %w", f.File, err)
	}
	did, exists := asMap[organizationIdentifier]
	if !exists {
		return "", fmt.Errorf("unable to find DID for organization identifier: %s", organizationIdentifier)
	}
	return did, nil
}
