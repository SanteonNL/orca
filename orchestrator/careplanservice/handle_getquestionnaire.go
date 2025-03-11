package careplanservice

import (
	"context"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleGetQuestionnaire(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.Questionnaire, error) {
	var questionnaire fhir.Questionnaire
	err := s.fhirClient.ReadWithContext(ctx, "Questionnaire/"+id, &questionnaire, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	return &questionnaire, nil
}
