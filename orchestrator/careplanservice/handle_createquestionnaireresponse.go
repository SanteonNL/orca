package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreateQuestionnaireResponseAuthzPolicy(profile profile.Provider) Policy[fhir.QuestionnaireResponse] {
	return LocalOrganizationPolicy[fhir.QuestionnaireResponse]{
		profile: profile,
	}
}
