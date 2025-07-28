package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func CreatePatientAuthzPolicy(profile profile.Provider) Policy[*fhir.Patient] {
	return LocalOrganizationPolicy[*fhir.Patient]{
		profile: profile,
	}
}

func UpdatePatientAuthzPolicy() Policy[*fhir.Patient] {
	return CreatorPolicy[*fhir.Patient]{}
}

func ReadPatientAuthzPolicy(fhirClientFactory FHIRClientFactory) Policy[*fhir.Patient] {
	return AnyMatchPolicy[*fhir.Patient]{
		Policies: []Policy[*fhir.Patient]{
			RelatedResourcePolicy[*fhir.Patient, *fhir.CarePlan]{
				fhirClientFactory:     fhirClientFactory,
				relatedResourcePolicy: CareTeamMemberPolicy[fhir.CarePlan]{},
				relatedResourceSearchParams: func(ctx context.Context, resource *fhir.Patient) (resourceType string, searchParams url.Values) {
					return "CarePlan", url.Values{"subject": []string{"Patient/" + *resource.Id}}
				},
			},
			CreatorPolicy[*fhir.Patient]{},
		},
	}
}
