package taskengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
)

type QuestionnaireResponseValidator interface {
	// Validate validates a QuestionnaireResponse against a Questionnaire.
	// If validation yields error or fatal issues, a non-nil error is returned. It will also return the OperationOutcome describing the issues.
	// If it couldn't perform validation, it returns no OperationOutcome and a non-nil error.
	Validate(ctx context.Context, questionnaire *fhir.Questionnaire, questionnaireResponse *fhir.QuestionnaireResponse) (*fhir.OperationOutcome, error)
}

// FHIRValidatorAPIQuestionnaireResponseValidator is a QuestionnaireResponseValidator that uses the HAPI FHIR Validator CLI to validate a QuestionnaireResponse against a Questionnaire.
// See https://confluence.hl7.org/spaces/FHIR/pages/35718580/Using+the+FHIR+Validator
type FHIRValidatorAPIQuestionnaireResponseValidator struct {
	client *http.Client
	URL    string
}

func (f FHIRValidatorAPIQuestionnaireResponseValidator) Validate(ctx context.Context, questionnaire *fhir.Questionnaire, questionnaireResponse *fhir.QuestionnaireResponse) (*fhir.OperationOutcome, error) {
	questionnaireJson, err := json.Marshal(questionnaire)
	questionnaireResponseJson, err := json.Marshal(questionnaireResponse)

	_, err = f.postQuestionnaireResource(ctx, err, questionnaireJson, "/Questionnaire")

	bodyBytes, err := f.postQuestionnaireResource(ctx, err, questionnaireResponseJson, "/QuestionnaireResponse")
	if err != nil {
		return nil, fmt.Errorf("failed to post QuestionnaireResponse: %w", err)
	}
	var createdQuestionnaireResponse fhir.QuestionnaireResponse
	if err = json.Unmarshal(bodyBytes, &createdQuestionnaireResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal QuestionnaireResponse: %w", err)
	}

	path := "QuestionnaireResponse/" + *createdQuestionnaireResponse.Id + "/$validate?questionnaire=required&display-issues-are-warnings=false"

	validationRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create validation request: %w", err)
	}
	validationRequest.Header.Add("Content-Type", "application/fhir+json")

	response, err := f.client.Do(validationRequest)

	if err != nil {
		return nil, fmt.Errorf("failed to send validation request: %w", err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read error response body: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to post with status code %d: %s", response.StatusCode, string(bodyBytes))
	}

	var bundle fhir.OperationOutcome
	if err = json.Unmarshal(body, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal OperationOutcome: %w", err)
	}

	for _, issue := range bundle.Issue {
		if issue.Severity == fhir.IssueSeverityFatal || issue.Severity == fhir.IssueSeverityError {
			return &bundle, errors.New("validation failed, see OperationOutcome for details")
		}
	}
	return &bundle, nil
}

func (f FHIRValidatorAPIQuestionnaireResponseValidator) postQuestionnaireResource(ctx context.Context, err error, questionnaireJson []byte, path string) ([]byte, error) {
	url := f.URL + path
	post, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(questionnaireJson))

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	post.Header.Add("Content-Type", "application/fhir+json")

	response, err := f.client.Do(post)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	bodyBytes, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read error response body: %w", err)
	}

	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to post with status code %d: %s", response.StatusCode, string(bodyBytes))
	}
	return bodyBytes, nil
}
