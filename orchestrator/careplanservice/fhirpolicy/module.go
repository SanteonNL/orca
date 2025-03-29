package fhirpolicy

import (
	_ "embed"

	"github.com/SanteonNL/orca/orchestrator/lib/policy"
)

//go:embed fhirpolicy.rego
var Source string

var Module = policy.RegoModule{
	Package: "fhirpolicy",
	Source:  Source,
}
