package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreateConditionAuthzPolicy(profile profile.Provider) Policy[fhir.Condition] {
	return LocalOrganizationPolicy[fhir.Condition]{
		profile: profile,
	}
}
