package taskengine

import "errors"

var ErrWorkflowNotFound = errors.New("workflow not found")

// Workflows is a map of workflow IDs to workflows.
type Workflows map[string]Workflow

// DefaultWorkflows returns a set of default, embedded workflows.
func DefaultWorkflows() Workflows {
	return Workflows{
		// COPD
		"2.16.528.1.1007.3.3.21514.ehr.orders|99534756439": Workflow{
			Steps: []WorkflowStep{
				{
					QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017",
				},
				//TODO: Commented out fow now, remove once we provide the Patient resource
				// {
				// 	QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017",
				// },
			},
		},
		// Hartfalen TODO: Made op the code value - check where it comes from
		"2.16.528.1.1007.3.3.21514.ehr.orders|99534756440": Workflow{
			Steps: []WorkflowStep{
				{
					QuestionnaireUrl: "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134018",
				},
			},
		},
		"tmp|fractuur-pols": Workflow{
			Steps: []WorkflowStep{
				{
					QuestionnaireUrl: "http://tmp.sharedcareplanning.nl/fhir/Questionnaire/fractuur-pols",
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
