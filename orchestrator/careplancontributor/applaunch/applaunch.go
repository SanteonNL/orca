package applaunch

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/external"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"net/http"
)

type Service interface {
	RegisterHandlers(mux *http.ServeMux)
	CreateEHRProxies() (map[string]coolfhir.HttpProxy, map[string]fhirclient.Client)
}

type Config struct {
	SmartOnFhir  smartonfhir.Config         `koanf:"sof"`
	Demo         demo.Config                `koanf:"demo"`
	ZorgPlatform zorgplatform.Config        `koanf:"zorgplatform"`
	External     map[string]external.Config `koanf:"external"`
}

func (c Config) Validate() error {
	return c.SmartOnFhir.Validate()
}

func DefaultConfig() Config {
	return Config{
		ZorgPlatform: zorgplatform.DefaultConfig(),
		SmartOnFhir:  smartonfhir.DefaultConfig(),
	}
}
