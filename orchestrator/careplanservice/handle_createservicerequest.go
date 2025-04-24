package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreateServiceRequestAuthzPolicy(profile profile.Provider) Policy[fhir.ServiceRequest] {
	return LocalOrganizationPolicy[fhir.ServiceRequest]{
		profile: profile,
	}
}
