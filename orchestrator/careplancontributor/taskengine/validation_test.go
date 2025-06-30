package taskengine

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine/testdata"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"testing"
)

func TestFHIRValidatorAPIQuestionnaireResponseValidator_Validate(t *testing.T) {
	ctx := context.Background()

	validator := FHIRValidatorAPIQuestionnaireResponseValidator{URL: "http://localhost:9090/fhir/", client: &http.Client{}}
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
