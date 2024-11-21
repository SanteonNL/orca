package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func validatePatientUpdate(a *fhir.Patient, b *fhir.Patient) error {
	return nil
}

func (s *Service) handleUpdatePatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return handleMetaBasedResourceUpdate[fhir.Patient](s, "Patient", ctx, request, tx, validatePatientUpdate)
}
