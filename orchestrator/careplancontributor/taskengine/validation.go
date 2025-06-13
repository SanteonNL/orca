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

type CliContext struct {
	SV     string   `json:"sv"`
	IG     []string `json:"ig"`
	Locale string   `json:"locale"`
}

type FileToValidate struct {
	FileName    string `json:"fileName"`
	FileContent string `json:"fileContent"`
	FileType    string `json:"fileType"`
}

type ValidateQuestionnaireRequest struct {
	CliContext      CliContext       `json:"cliContext"`
	FilesToValidate []FileToValidate `json:"filesToValidate"`
}

func (f FHIRValidatorAPIQuestionnaireResponseValidator) Validate(ctx context.Context, questionnaire *fhir.Questionnaire, questionnaireResponse *fhir.QuestionnaireResponse) (*fhir.OperationOutcome, error) {
	questionnaireJson, err := json.Marshal(questionnaire)
	questionnaireResponseJson, err := json.Marshal(questionnaireResponse)

	requestBody := ValidateQuestionnaireRequest{
		CliContext: CliContext{
			SV:     "4.0.1",
			IG:     []string{"hl7.fhir.us.core#4.0.0"},
			Locale: "en",
		},
		FilesToValidate: []FileToValidate{
			{
				FileName:    "questionnaire.json",
				FileContent: string(questionnaireJson),
				FileType:    "json",
			},
			{
				FileName:    "questionnaireResponse.json",
				FileContent: string(questionnaireResponseJson),
				FileType:    "json",
			},
		},
	}

	data, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, f.URL, bytes.NewReader(data))

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Add("Content-Type", "application/json")

	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	bodyBytes, err := io.ReadAll(response.Body)
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		if err != nil {
			return nil, fmt.Errorf("failed to read error response body: %w", err)
		}
		return nil, fmt.Errorf("validation failed with status code %d: %s", response.StatusCode, string(bodyBytes))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var bundle fhir.OperationOutcome
	if err = json.Unmarshal(bodyBytes, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation result: %w", err)
	}
	for _, issue := range bundle.Issue {
		if issue.Severity == fhir.IssueSeverityFatal || issue.Severity == fhir.IssueSeverityError {
			return &bundle, errors.New("validation failed, see OperationOutcome for details")
		}
	}
	return &bundle, nil

}
