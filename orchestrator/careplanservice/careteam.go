package careplanservice

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) updateCareTeam(carePlan *fhir.CarePlan) ([]fhir.BundleEntry, error) {

	s.fhirClient.Read("CarePlan/"+*carePlan.Id, carePLan, fhirclient.QueryParam("include", "CarePlan:careTeam"))
}
