package taskengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine/testdata"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"strings"
	"testing"
)

func LoadTestQuestionnairesAndHealthcareSevices(t *testing.T, client fhirclient.Client) {
	var healthcareServiceBundle fhir.Bundle
	data, err := testdata.FS.ReadFile("healthcareservice-bundle.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &healthcareServiceBundle))
	require.NoError(t, client.Create(healthcareServiceBundle, &healthcareServiceBundle, fhirclient.AtPath("/")))

	var questionnaireBundle fhir.Bundle
	data, err = testdata.FS.ReadFile("questionnaire-bundle.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &questionnaireBundle))
	require.NoError(t, client.Create(questionnaireBundle, &questionnaireBundle, fhirclient.AtPath("/")))

	data, err = testdata.FS.ReadFile("copd-asthma-questionnaire-bundle.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &questionnaireBundle))
	require.NoError(t, client.Create(questionnaireBundle, &questionnaireBundle, fhirclient.AtPath("/")))
}

func DefaultTestWorkflowProvider() TestWorkflowProvider {
	return TestWorkflowProvider{
		// Telemonitoring
		"http://snomed.info/sct|719858009": map[string]Workflow{
			// COPD
			"http://snomed.info/sct|13645005": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://example.com/fhir/Questionnaire/questionnaire-copd",
					},
				},
			},
			// Heart failure
			"http://snomed.info/sct|84114007": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://example.com/fhir/Questionnaire/zbj-questionnaire-telemonitoring-heartfailure-enrollment",
					},
				},
			},
			// Asthma
			"http://snomed.info/sct|195967001": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://example.com/fhir/Questionnaire/questionnaire-asthma",
					},
				},
			},
		},
	}
}

// TestWorkflowProvider is an in-memory WorkflowProvider.
// It's a map of a care service (e.g. Telemonitoring, http://snomed.info/sct|719858009),
// to conditions (e.g. COPD, http://snomed.info/sct|13645005) and their workflows.
type TestWorkflowProvider map[string]map[string]Workflow

var _ WorkflowProvider = TestWorkflowProvider{}

func (m TestWorkflowProvider) QuestionnaireLoader() QuestionnaireLoader {
	return TestQuestionnaireLoader{}
}

func (m TestWorkflowProvider) Provide(_ context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error) {
	if serviceCode.System == nil || serviceCode.Code == nil || conditionCode.System == nil || conditionCode.Code == nil {
		return nil, errors.New("serviceCode and conditionCode must have a system and code")
	}
	if workflows, ok := m[*serviceCode.System+"|"+*serviceCode.Code]; ok {
		if workflow, ok := workflows[*conditionCode.System+"|"+*conditionCode.Code]; ok {
			return &workflow, nil
		}
		return nil, errors.Join(ErrWorkflowNotFound, fmt.Errorf("condition code does not match any conditions (service=%s|%s, condition=%s|%s)", *serviceCode.System, *serviceCode.Code, *conditionCode.System, *conditionCode.Code))
	}
	return nil, errors.Join(ErrWorkflowNotFound, errors.New("service code does not match any offered services"))
}

var _ QuestionnaireLoader = TestQuestionnaireLoader{}

type TestQuestionnaireLoader struct {
}

func (t TestQuestionnaireLoader) Load(ctx context.Context, u string) (*fhir.Questionnaire, error) {
	var normativeQuestionnaireBundle fhir.Bundle
	data, err := testdata.FS.ReadFile("questionnaire-bundle.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &normativeQuestionnaireBundle); err != nil {
		return nil, err
	}

	var questionnaireBundle fhir.Bundle
	data, err = testdata.FS.ReadFile("copd-asthma-questionnaire-bundle.json")
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &questionnaireBundle); err != nil {
		return nil, err
	}
	questionnaireBundle.Entry = append(questionnaireBundle.Entry, normativeQuestionnaireBundle.Entry...)

	id := u[strings.LastIndex(u, "/")+1:] // Take ID of the FHIR resource from the URL
	var result fhir.Questionnaire
	if err := coolfhir.ResourceInBundle(&questionnaireBundle, coolfhir.EntryHasID(id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}
