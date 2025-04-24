package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func UpdateServiceRequestAuthzPolicy() Policy[fhir.ServiceRequest] {
	return CreatorPolicy[fhir.ServiceRequest]{}
}
