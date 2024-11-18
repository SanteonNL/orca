package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleGetQuestionnaireResponse(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.QuestionnaireResponse, error) {
	var questionnaireResponse fhir.QuestionnaireResponse
	err := s.fhirClient.Read("QuestionnaireResponse/"+id, &questionnaireResponse, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	err = s.handleTaskBasedResourceAuth(ctx, headers, questionnaireResponse.BasedOn, "QuestionnaireResponse")
	if err != nil {
		return nil, err
	}

	return &questionnaireResponse, nil
}
