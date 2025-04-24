package careplanservice

import (
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func UpdateConditionAuthzPolicy() Policy[fhir.Condition] {
	return CreatorPolicy[fhir.Condition]{}
}
