package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func UpdatePatientAuthzPolicy() Policy[fhir.Patient] {
	return CreatorPolicy[fhir.Patient]{}
}
