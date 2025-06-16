package taskengine

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine/testdata"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"os"
	"testing"
)

func TestFHIRValidatorAPIQuestionnaireResponseValidator_Validate(t *testing.T) {
	ctx := context.Background()

	validator := FHIRValidatorAPIQuestionnaireResponseValidator{URL: "http://localhost:3500/validate", client: &http.Client{}}
	t.Run("ok", func(t *testing.T) {
		questionnaireData, err := testdata.FS.ReadFile("questionnaire-heartfailure-enrollment.json")
		require.NoError(t, err)
		questionnaire := &fhir.Questionnaire{}
		err = json.Unmarshal(questionnaireData, questionnaire)
		require.NoError(t, err)

		questionnaireResponseData, err := testdata.FS.ReadFile("questionnaire-heartfailure-enrollment-response-ok.json")
		require.NoError(t, err)
		questionnaireResponse := &fhir.QuestionnaireResponse{}
		err = json.Unmarshal(questionnaireResponseData, questionnaireResponse)
		require.NoError(t, err)

		ooc, err := validator.Validate(ctx, questionnaire, questionnaireResponse)
		require.NoError(t, err)
		require.NotNil(t, ooc)
	})
	t.Run("errors", func(t *testing.T) {
		questionnaireData, err := testdata.FS.ReadFile("questionnaire-heartfailure-enrollment.json")
		require.NoError(t, err)
		questionnaire := &fhir.Questionnaire{}
		err = json.Unmarshal(questionnaireData, questionnaire)
		require.NoError(t, err)

		questionnaireResponseData, err := testdata.FS.ReadFile("questionnaire-heartfailure-enrollment-response-nok.json")
		require.NoError(t, err)
		questionnaireResponse := &fhir.QuestionnaireResponse{}
		err = json.Unmarshal(questionnaireResponseData, questionnaireResponse)
		require.NoError(t, err)

		ooc, err := validator.Validate(ctx, questionnaire, questionnaireResponse)
		require.Error(t, err)
		require.NotNil(t, ooc)
		oocJSON, _ := json.Marshal(ooc)
		require.Contains(t, string(oocJSON), "Item has answer, even though it is not enabled (item id = '170292e5-3163-43b4-88af-affb3e4c27ab')")
	})
}

func downloadFHIRValidatorCLI() error {
	validatorJar := "bin/fhir_validator_cli-6.5.7.jar"
	// If not exists, download it from https://github.com/hapifhir/org.hl7.fhir.core/releases/download/6.5.7/validator_cli.jar
	if _, err := os.Stat(validatorJar); os.IsNotExist(err) {
		// download it
		if err := os.MkdirAll("bin", 0755); err != nil {
			return err
		}
		println("Downloading FHIR Validator CLI...")
		httpResponse, err := http.Get("https://github.com/hapifhir/org.hl7.fhir.core/releases/download/6.5.7/validator_cli.jar")
		if err != nil {
			return err
		}
		defer httpResponse.Body.Close()
		file, err := os.Create(validatorJar)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := io.Copy(file, httpResponse.Body); err != nil {
			return err
		}
	}
	return nil
}
