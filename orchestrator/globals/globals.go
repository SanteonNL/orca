package globals

import (
	"crypto/tls"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.DefaultContextLogger = &log.Logger
}

var CarePlanServiceFhirClient fhirclient.Client

// StrictMode is a global variable that can be set to true to enable strict mode. If strict mode is enabled,
// potentially unsafe behavior is disabled.
var StrictMode bool

// DefaultTLSConfig returns a default, secure TLS configuration. If you want to customize the configuration for a specific
// TLS client, you should clone it before doing so.
var DefaultTLSConfig = &tls.Config{
	MinVersion: tls.VersionTLS12,
}
