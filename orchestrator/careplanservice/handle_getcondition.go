package careplanservice

import (
	"context"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleGetCondition(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Condition, error) {
	var condition fhir.Condition
	err := s.fhirClient.ReadWithContext(ctx, "Condition/"+id, &condition, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier == nil || condition.Subject.Identifier.System == nil || condition.Subject.Identifier.Value == nil {
		log.Ctx(ctx).Warn().Msg("Condition does not have Patient as subject, can't verify access")
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Condition",
			StatusCode: http.StatusForbidden,
		}
	}

	return &condition, nil
}
