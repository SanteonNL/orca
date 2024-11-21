package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func validateQuestionnaireResponseUpdate(a *fhir.QuestionnaireResponse, b *fhir.QuestionnaireResponse) error {
	// The only valid updates are status and item, ensure that the objects are equal besides those fields
	comparisonA := deep.Copy(a)
	comparisonB := deep.Copy(b)
	comparisonA.Status = comparisonB.Status
	comparisonA.Item = comparisonB.Item
	if !deep.Equal(comparisonA, comparisonB) {
		return &coolfhir.ErrorWithCode{
			Message:    "QuestionnaireResponse fields other than Status and Item cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	return nil
}

func (s *Service) handleUpdateQuestionnaireResponse(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return handleMetaBasedResourceUpdate[fhir.QuestionnaireResponse](s, "QuestionnaireResponse", ctx, request, tx, validateQuestionnaireResponseUpdate)
}
