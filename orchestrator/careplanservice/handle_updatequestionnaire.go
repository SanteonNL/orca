package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func UpdateQuestionnaireAuthzPolicy() Policy[fhir.Questionnaire] {
	return EveryoneHasAccessPolicy[fhir.Questionnaire]{}
}
