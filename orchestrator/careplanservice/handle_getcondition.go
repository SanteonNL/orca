package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"strings"
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
	if condition.Subject.Reference != nil && strings.HasPrefix(*condition.Subject.Reference, "Patient/") {
		_, err = s.handleGetPatient(ctx, strings.TrimPrefix(*condition.Subject.Reference, "Patient/"), headers)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: How to handle condition.Subject being of type Group?
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Condition does not have Patient as subject, can't verify access",
			StatusCode: http.StatusForbidden,
		}
	}

	return &condition, nil
}
