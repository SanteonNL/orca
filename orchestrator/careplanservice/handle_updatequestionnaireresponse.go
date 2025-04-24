package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func UpdateQuestionnaireResponseAuthzPolicy() Policy[fhir.QuestionnaireResponse] {
	return CreatorPolicy[fhir.QuestionnaireResponse]{}
}
