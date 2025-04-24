package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func ReadConditionAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Condition] {
	// TODO: Find out new auth requirements for condition
	return AnyMatchPolicy[fhir.Condition]{
		Policies: []Policy[fhir.Condition]{
			RelatedResourceSearchPolicy[fhir.Condition, fhir.Patient]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadPatientAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.Condition) (string, *url.Values) {
					if resource.Subject.Identifier == nil || resource.Subject.Identifier.System == nil || resource.Subject.Identifier.Value == nil {
						log.Ctx(ctx).Warn().Msg("Condition does not have Patient as subject, can't verify access")
						return "Patient", nil
					}
					return "Patient", &url.Values{
						"identifier": []string{fmt.Sprintf("%s|%s", *resource.Subject.Identifier.System, *resource.Subject.Identifier.Value)},
					}
				},
			},
			CreatorPolicy[fhir.Condition]{},
		},
	}
}
