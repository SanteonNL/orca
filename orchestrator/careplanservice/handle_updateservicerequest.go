package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func validateServiceRequestUpdate(a *fhir.ServiceRequest, b *fhir.ServiceRequest) error {
	if !deep.Equal(a.BasedOn, b.BasedOn) {
		return &coolfhir.ErrorWithCode{
			Message:    "ServiceRequest.BasedOn cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	if !deep.Equal(a.Subject, b.Subject) {
		return &coolfhir.ErrorWithCode{
			Message:    "ServiceRequest.Subject cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	if !deep.Equal(a.Requester, b.Requester) {
		return &coolfhir.ErrorWithCode{
			Message:    "ServiceRequest.Requester cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	if !deep.Equal(a.Performer, b.Performer) {
		return &coolfhir.ErrorWithCode{
			Message:    "ServiceRequest.Performer cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	if !deep.Equal(a.PerformerType, b.PerformerType) {
		return &coolfhir.ErrorWithCode{
			Message:    "ServiceRequest.PerformerType cannot be updated",
			StatusCode: http.StatusBadRequest,
		}
	}
	return nil
}

func (s *Service) handleUpdateServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return handleMetaBasedResourceUpdate[fhir.ServiceRequest](s, "ServiceRequest", ctx, request, tx, validateServiceRequestUpdate)
}
