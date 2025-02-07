package taskengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"time"
)

type QuestionnaireResponseValidator interface {
	// Validate validates a QuestionnaireResponse against a Questionnaire.
	// If validation yields error or fatal issues, a non-nil error is returned. It will also return the OperationOutcome describing the issues.
	// If it couldn't perform validation, it returns no OperationOutcome and a non-nil error.
	Validate(ctx context.Context, questionnaire *fhir.Questionnaire, questionnaireResponse *fhir.QuestionnaireResponse) (*fhir.OperationOutcome, error)
}

// FHIRValidatorCLIQuestionnaireResponseValidator is a QuestionnaireResponseValidator that uses the HAPI FHIR Validator CLI to validate a QuestionnaireResponse against a Questionnaire.
// See https://confluence.hl7.org/spaces/FHIR/pages/35718580/Using+the+FHIR+Validator
type FHIRValidatorCLIQuestionnaireResponseValidator struct {
	JARFile string
}

func (f FHIRValidatorCLIQuestionnaireResponseValidator) Validate(ctx context.Context, questionnaire *fhir.Questionnaire, questionnaireResponse *fhir.QuestionnaireResponse) (*fhir.OperationOutcome, error) {
	const fileMode = fs.ModePerm
	workingDirectory, err := os.MkdirTemp("", "fhir-validator-cli")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workingDirectory); err != nil {
			log.Ctx(ctx).Err(err).Msgf("Failed to remove temporary directory (path=%s)", workingDirectory)
		}
	}()

	// Write Questionnaire as JSON to file
	questionnaireFile := path.Join(workingDirectory, "ig", "questionnaire.json")
	if err = os.Mkdir(path.Dir(questionnaireFile), fileMode); err != nil {
		return nil, err
	}
	if err := marshalJSONToFile(questionnaireFile, questionnaire); err != nil {
		return nil, fmt.Errorf("failed to write Questionnaire to file: %w", err)
	}
	// Write QuestionnaireResponse to file
	questionnaireResponseFile := path.Join(workingDirectory, "questionnaire-response.json")
	if err = marshalJSONToFile(questionnaireResponseFile, questionnaireResponse); err != nil {
		return nil, fmt.Errorf("failed to write QuestionnaireResponse to file: %w", err)
	}
	// Result file
	validationResultFile := path.Join(workingDirectory, "result.json")
	// Execute validator CLI: java -jar /fhir_validator_cli.jar questionnaire-response.json -version 4.0.1 -ig ./ig -questionnaire required
	binary, err := exec.LookPath("java")
	if err != nil {
		return nil, fmt.Errorf("couldn't find java in PATH: %w", err)
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.Command(binary,
		"-jar", f.JARFile,
		questionnaireResponseFile,
		"-version", "4.0.1",
		"-ig", path.Dir(questionnaireFile),
		"-display-issues-are-warnings",
		"-output", validationResultFile,
		"-output-style", "json",
		"-questionnaire", "required")
	cmd.WaitDelay = time.Minute // timeout 10s
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	var resultJSON []byte
	if err := cmd.Run(); err != nil {
		// If there's a result file, validation was performed but there are error or fatal issues
		resultJSON, err = os.ReadFile(validationResultFile)
		if err != nil {
			// Couldn't read result file, so validator CLI failed
			log.Ctx(ctx).Warn().Err(err).Msgf("Failed to execute FHIR validator CLI, output: %s", stdout.String()+stderr.String())
			return nil, fmt.Errorf("failed to execute FHIR validator CLI: %w", err)
		}
	}
	log.Ctx(ctx).Trace().Msgf("FHIR validator CLI output: %s", stdout.String()+stderr.String())
	resultJSON, err = os.ReadFile(validationResultFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read validation result file (file=%s): %w", validationResultFile, err)
	}
	var bundle fhir.OperationOutcome
	if err = json.Unmarshal(resultJSON, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal validation result: %w", err)
	}
	for _, issue := range bundle.Issue {
		if issue.Severity == fhir.IssueSeverityFatal || issue.Severity == fhir.IssueSeverityError {
			return &bundle, errors.New("validation failed, see OperationOutcome for details")
		}
	}
	return &bundle, nil
}

func HasValidationFailed(ctx context.Context, err error) error {
	return fmt.Errorf("validation failed: %w", err)
}

func marshalJSONToFile(questionnaireFile string, obj any) error {
	handle, err := os.Create(questionnaireFile)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err = json.NewEncoder(handle).Encode(obj); err != nil {
		return err
	}
	return nil
}
