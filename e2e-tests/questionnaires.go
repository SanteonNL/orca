package main

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/caramel/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func questionnaireResponseTo(questionnaireUrl string) fhir.QuestionnaireResponse {
	// TODO: This Response doesn't really fulfill the Questionnaire
	return questionnaireResponseTelemonitoring1InclusionCriteria(questionnaireUrl)
}

func questionnaireResponseTelemonitoring1InclusionCriteria(questionnaireUrl string) fhir.QuestionnaireResponse {
	return fhir.QuestionnaireResponse{
		Questionnaire: to.Ptr(questionnaireUrl),
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
