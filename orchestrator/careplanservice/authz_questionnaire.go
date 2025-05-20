package careplanservice

import "github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

func CreateQuestionnaireAuthzPolicy() Policy[*fhir.Questionnaire] {
	return AnyonePolicy[*fhir.Questionnaire]{}
}

func UpdateQuestionnaireAuthzPolicy() Policy[*fhir.Questionnaire] {
	return AnyonePolicy[*fhir.Questionnaire]{}
}

func ReadQuestionnaireAuthzPolicy() Policy[*fhir.Questionnaire] {
	return AnyonePolicy[*fhir.Questionnaire]{}
}

// DeleteQuestionnaireAuthzPolicy returns a policy that always allows deletion of Questionnaire resources.
func DeleteQuestionnaireAuthzPolicy() Policy[*fhir.Questionnaire] {
	return AnyonePolicy[*fhir.Questionnaire]{}
}
