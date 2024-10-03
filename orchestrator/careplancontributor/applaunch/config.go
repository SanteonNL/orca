package applaunch

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
)

type Config struct {
	SmartOnFhir  smartonfhir.Config  `koanf:"sof"`
	Demo         demo.Config         `koanf:"demo"`
	ZorgPlatform zorgplatform.Config `koanf:"zorgplatform"`
}
