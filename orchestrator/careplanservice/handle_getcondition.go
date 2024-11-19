package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func (s *Service) handleGetCondition(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Condition, error) {
	var condition fhir.Condition
	err := s.fhirClient.Read("Condition/"+id, &condition, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier != nil && condition.Subject.Identifier.System != nil && condition.Subject.Identifier.Value != nil {
		bundle, err := s.handleSearchPatient(ctx, map[string][]string{"identifier": {fmt.Sprintf("%s|%s", *condition.Subject.Identifier.System, *condition.Subject.Identifier.Value)}}, headers)
		if err != nil {
			return nil, err
		}
		if len(bundle.Entry) == 0 {
			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			}
		}
	} else {
		log.Warn().Msg("Condition does not have Patient as subject, can't verify access")
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Condition",
			StatusCode: http.StatusForbidden,
		}
	}

	return &condition, nil
}
