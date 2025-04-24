package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func ReadPatientAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Patient] {
	return RelatedResourceSearchPolicy[fhir.Patient, fhir.CarePlan]{
		fhirClient:            fhirClient,
		relatedResourcePolicy: CareTeamMemberPolicy[fhir.CarePlan]{},
		relatedResourceSearchParams: func(ctx context.Context, resource fhir.Patient) (resourceType string, searchParams *url.Values) {
			return "CarePlan", &url.Values{"subject": []string{"Patient/" + *resource.Id}}
		},
	}
}
