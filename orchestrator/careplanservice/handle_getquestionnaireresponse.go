package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func ReadQuestionnaireResponseAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.QuestionnaireResponse] {
	return AnyMatchPolicy[fhir.QuestionnaireResponse]{
		Policies: []Policy[fhir.QuestionnaireResponse]{
			RelatedResourceSearchPolicy[fhir.QuestionnaireResponse, fhir.Task]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadTaskAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.QuestionnaireResponse) (resourceType string, searchParams *url.Values) {
					return "Task", &url.Values{"output-reference": []string{"QuestionnaireResponse/" + *resource.Id}}
				},
			},
			CreatorPolicy[fhir.QuestionnaireResponse]{},
		},
	}
}
