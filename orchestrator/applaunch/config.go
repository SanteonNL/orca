package applaunch

import (
	"github.com/SanteonNL/orca/orchestrator/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/applaunch/smartonfhir"
)

type Config struct {
	SmartOnFhir smartonfhir.Config `koanf:"sof"`
	Demo        demo.Config        `koanf:"demo"`
}
