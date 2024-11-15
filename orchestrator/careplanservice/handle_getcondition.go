package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func (s *Service) handleGetCondition(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Condition, error) {
	// Verify requester is authenticated
	_, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var condition fhir.Condition
	err = s.fhirClient.Read("Condition/"+id, &condition, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier != nil && *condition.Subject.Identifier.System == "http://fhir.nl/fhir/NamingSystem/bsn" {
		bundle, err := s.handleSearchPatient(ctx, map[string][]string{"identifier": {patientBSNFromIdentifier(condition.Subject.Identifier)}}, headers)
		if err != nil {
			return nil, err
		}
		if len(bundle.Entry) == 0 {
			return nil, &coolfhir.ErrorWithCode{
				Message:    "User does not have access to Condition",
				StatusCode: http.StatusUnauthorized,
			}
		}
	} else {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Condition does not have Patient as subject, can't verify access",
			StatusCode: http.StatusForbidden,
		}
	}

	return &condition, nil
}
