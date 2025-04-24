package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreatePatientAuthzPolicy(profile profile.Provider) Policy[fhir.Patient] {
	return LocalOrganizationPolicy[fhir.Patient]{
		profile: profile,
	}
}
