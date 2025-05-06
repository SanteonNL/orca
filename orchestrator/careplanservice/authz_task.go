package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func ReadTaskAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Task] {
	return AnyMatchPolicy[fhir.Task]{
		Policies: []Policy[fhir.Task]{
			TaskOwnerOrRequesterPolicy[fhir.Task]{},
			RelatedResourcePolicy[fhir.Task, fhir.CarePlan]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: CareTeamMemberPolicy[fhir.CarePlan]{},
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.Task) (string, *url.Values) {
					var ids []string
					for _, reference := range resource.BasedOn {
						if reference.Reference != nil {
							ids = append(ids, getResourceID(*reference.Reference))
						}
					}
					if len(ids) == 0 {
						return "", nil
					}
					return "CarePlan", &url.Values{
						"_id": ids,
					}
				},
			},
		},
	}
}
