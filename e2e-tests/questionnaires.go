package main

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func questionnaireResponseTo(questionnaire fhir.Questionnaire) fhir.QuestionnaireResponse {
	switch *questionnaire.Url {
	case "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017":
		return questionnaireResponseTelemonitoring1InclusionCriteria(questionnaire)
	default:
		panic("unsupported questionnaire: " + *questionnaire.Url)
	}

}

func questionnaireResponseTelemonitoring1InclusionCriteria(questionnaire fhir.Questionnaire) fhir.QuestionnaireResponse {
	return fhir.QuestionnaireResponse{
		Questionnaire: questionnaire.Url,
		Status:        fhir.QuestionnaireResponseStatusCompleted,
		Item: []fhir.QuestionnaireResponseItem{
			{
				LinkId: "2.16.840.1.113883.2.4.3.11.60.909.2.2.2208",
				Answer: []fhir.QuestionnaireResponseItemAnswer{
					{
						ValueBoolean: to.Ptr(true),
					},
				},
			},
			{
				LinkId: "2.16.840.1.113883.2.4.3.11.60.909.2.2.2209",
				Answer: []fhir.QuestionnaireResponseItemAnswer{
					{
						ValueBoolean: to.Ptr(true),
					},
				},
			},
			{
				LinkId: "2.16.840.1.113883.2.4.3.11.60.909.2.2.2210",
				Answer: []fhir.QuestionnaireResponseItemAnswer{
					{
						ValueBoolean: to.Ptr(true),
					},
				},
			},
			{
				LinkId: "2.16.840.1.113883.2.4.3.11.60.909.2.2.2211",
				Answer: []fhir.QuestionnaireResponseItemAnswer{
					{
						ValueBoolean: to.Ptr(true),
					},
				},
			},
			{
				LinkId: "2.16.840.1.113883.2.4.3.11.60.909.2.2.2212",
				Answer: []fhir.QuestionnaireResponseItemAnswer{
					{
						ValueBoolean: to.Ptr(true),
					},
				},
			},
		},
	}
}
