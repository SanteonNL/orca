package careplanservice

import "github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

func ReadCarePlanAuthzPolicy() Policy[*fhir.CarePlan] {
	return CareTeamMemberPolicy[fhir.CarePlan]{}
}
