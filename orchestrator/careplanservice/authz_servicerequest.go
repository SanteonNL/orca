package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

func CreateServiceRequestAuthzPolicy(profile profile.Provider) Policy[fhir.ServiceRequest] {
	return LocalOrganizationPolicy[fhir.ServiceRequest]{
		profile: profile,
	}
}

func UpdateServiceRequestAuthzPolicy() Policy[fhir.ServiceRequest] {
	return CreatorPolicy[fhir.ServiceRequest]{}
}

func ReadServiceRequestAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.ServiceRequest] {
	return AnyMatchPolicy[fhir.ServiceRequest]{
		Policies: []Policy[fhir.ServiceRequest]{
			RelatedResourcePolicy[fhir.ServiceRequest, fhir.Task]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadTaskAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.ServiceRequest) (string, *url.Values) {
					return "Task", &url.Values{"focus": []string{"ServiceRequest/" + *resource.Id}}
				},
			},
			CreatorPolicy[fhir.ServiceRequest]{},
		},
	}
}
