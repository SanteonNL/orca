package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func CreateQuestionnaireResponseAuthzPolicy(profile profile.Provider) Policy[*fhir.QuestionnaireResponse] {
	return LocalOrganizationPolicy[*fhir.QuestionnaireResponse]{
		profile: profile,
	}
}

func UpdateQuestionnaireResponseAuthzPolicy() Policy[*fhir.QuestionnaireResponse] {
	return CreatorPolicy[*fhir.QuestionnaireResponse]{}
}

func ReadQuestionnaireResponseAuthzPolicy(fhirClient fhirclient.Client) Policy[*fhir.QuestionnaireResponse] {
	return AnyMatchPolicy[*fhir.QuestionnaireResponse]{
		Policies: []Policy[*fhir.QuestionnaireResponse]{
			RelatedResourcePolicy[*fhir.QuestionnaireResponse, *fhir.Task]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadTaskAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource *fhir.QuestionnaireResponse) (resourceType string, searchParams url.Values) {
					return "Task", url.Values{"output-reference": []string{"QuestionnaireResponse/" + *resource.Id}}
				},
			},
			CreatorPolicy[*fhir.QuestionnaireResponse]{},
		},
	}
}

func DeleteQuestionnaireResponseAuthzPolicy() Policy[*fhir.QuestionnaireResponse] {
	return AnyonePolicy[*fhir.QuestionnaireResponse]{}
}
