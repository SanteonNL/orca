package globals

import (
	"context"
	"crypto/tls"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.DefaultContextLogger = &log.Logger
}

func CreateCPSFHIRClient(ctx context.Context) (fhirclient.Client, error) {
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("create CPS FHIR client: %w", err)
	}
	fhirClient := cpsFHIRClientsByTenant[tenant.ID]
	if fhirClient == nil {
		return nil, fmt.Errorf("create CPS FHIR client: no client for tenant (id=%s)", tenant.ID)
	}
	return fhirClient, nil
}

func RegisterCPSFHIRClient(tenantID string, client fhirclient.Client) {
	if _, exists := cpsFHIRClientsByTenant[tenantID]; StrictMode && exists {
		panic(fmt.Sprintf("CPS FHIR client for tenant %s already exists", tenantID))
	}
	cpsFHIRClientsByTenant[tenantID] = client
}

var cpsFHIRClientsByTenant = make(map[string]fhirclient.Client)

// StrictMode is a global variable that can be set to true to enable strict mode. If strict mode is enabled,
// potentially unsafe behavior is disabled.
var StrictMode bool

// DefaultTLSConfig returns a default, secure TLS configuration. If you want to customize the configuration for a specific
// TLS client, you should clone it before doing so.
var DefaultTLSConfig = &tls.Config{
	MinVersion: tls.VersionTLS12,
}
