package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func ReadTaskAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Task] {
	return AnyMatchPolicy[fhir.Task]{
		Policies: []Policy[fhir.Task]{
			TaskOwnerOrRequesterPolicy[fhir.Task]{},
			RelatedResourcePolicy[fhir.Task, fhir.CarePlan]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: CareTeamMemberPolicy[fhir.CarePlan]{},
				relatedResourceRefs: func(ctx context.Context, resource fhir.Task) ([]string, error) {
					var refs []string
					for _, reference := range resource.BasedOn {
						if reference.Reference != nil {
							refs = append(refs, *reference.Reference)
						}
					}
					return refs, nil
				},
			},
		},
	}
}
