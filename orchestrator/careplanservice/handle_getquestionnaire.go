package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func ReadQuestionnaireAuthzPolicy() Policy[fhir.Questionnaire] {
	return EveryoneHasAccessPolicy[fhir.Questionnaire]{}
}
