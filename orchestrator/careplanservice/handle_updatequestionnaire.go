package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdateQuestionnaire(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return handleMetaBasedResourceUpdate[fhir.Questionnaire](s, "Questionnaire", ctx, request, tx, func(a *fhir.Questionnaire, b *fhir.Questionnaire) error {
		return nil
	})
}
