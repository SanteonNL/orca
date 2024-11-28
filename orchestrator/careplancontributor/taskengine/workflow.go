package taskengine

import (
	"context"
	"errors"
	"fmt"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

// WorkflowProvider provides workflows (a set of questionnaires required for accepting a Task) to the Task Filler.
type WorkflowProvider interface {
	// Provide returns the workflow for a given service and condition.
	// If no workflow is found, an error is returned.
	Provide(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error)
}

var _ WorkflowProvider = MemoryWorkflowProvider{}

// MemoryWorkflowProvider is an in-memory WorkflowProvider.
// It's a map of a care service (e.g. Telemonitoring, http://snomed.info/sct|719858009),
// to conditions (e.g. COPD, http://snomed.info/sct|13645005) and their workflows.
type MemoryWorkflowProvider map[string]map[string]Workflow

func (m MemoryWorkflowProvider) Provide(_ context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error) {
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

// DefaultWorkflows returns a set of default, embedded workflows.
func DefaultWorkflows() MemoryWorkflowProvider {
	return MemoryWorkflowProvider{
		// Telemonitoring
		"http://snomed.info/sct|719858009": map[string]Workflow{
			// COPD
			"http://snomed.info/sct|13645005": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017",
					},
					//TODO: Commented out fow now, remove once we provide the Patient resource
					//{
					//	QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017",
					//},
				},
			},
			// Heart failure
			"http://snomed.info/sct|84114007": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134018",
					},
				},
			},
			// Asthma
			"http://snomed.info/sct|195967001": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134019",
					},
				},
			},
			// TODO: what about this?
			"tmp|fractuur-pols": {
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "http://tmp.sharedcareplanning.nl/fhir/Questionnaire/fractuur-pols",
					},
				},
			},
		},
	}
}

type Workflow struct {
	Steps []WorkflowStep
}

func (w Workflow) Start() WorkflowStep {
	return w.Steps[0]
}

func (w Workflow) Proceed(previousQuestionnaireUrl string) (*WorkflowStep, error) {
	for i, step := range w.Steps {
		if step.QuestionnaireUrl == previousQuestionnaireUrl {
			if i+1 < len(w.Steps) {
				return &w.Steps[i+1], nil
			} else {
				return nil, nil
			}
		}
	}
	return nil, errors.New("previous questionnaire doesn't exist for this workflow")
}

type WorkflowStep struct {
	QuestionnaireUrl string
}
