package careplanservice

import (
	"context"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetQuestionnaire fetches the requested Questionnaire and validates if the requester is authenticated
func (s *Service) handleGetQuestionnaireResponse(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.QuestionnaireResponse, error) {
	var questionnaireResponse fhir.QuestionnaireResponse
	err := s.fhirClient.ReadWithContext(ctx, "QuestionnaireResponse/"+id, &questionnaireResponse, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// Fetch tasks where the QuestionnaireResponse is in the task Output
	bundle, err := s.handleSearchTask(ctx, url.Values{"output-reference": []string{"QuestionnaireResponse/" + id}}, headers)
	if err != nil {
		return nil, err
	}

	// If the user has access to the task, they have access to the questionnaire response
	if len(bundle.Entry) == 0 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to QuestionnaireResponse",
			StatusCode: http.StatusForbidden,
		}
	}

	return &questionnaireResponse, nil
}
