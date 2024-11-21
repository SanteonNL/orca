package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func validateConditionUpdate(a *fhir.Condition, b *fhir.Condition) error {
	if !deep.Equal(a.Subject, b.Subject) {
		return &coolfhir.ErrorWithCode{
			Message:    "Condition.Subject cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}

	return nil
}

func (s *Service) handleUpdateCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return handleMetaBasedResourceUpdate[fhir.Condition](s, "Condition", ctx, request, tx, validateConditionUpdate)
}
