package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func CreateQuestionnaireAuthzPolicy() Policy[fhir.Questionnaire] {
	return EveryoneHasAccessPolicy[fhir.Questionnaire]{}
}
