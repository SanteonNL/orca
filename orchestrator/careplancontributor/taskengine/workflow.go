package taskengine

import (
	"context"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

// WorkflowProvider provides workflows (a set of questionnaires required for accepting a Task) to the Task Filler.
type WorkflowProvider interface {
	// Provide returns the workflow for a given service and condition.
	// If no workflow is found, an error is returned.
	Provide(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error)
	QuestionnaireLoader() QuestionnaireLoader
}

var _ WorkflowProvider = FhirApiWorkflowProvider{}

// FhirApiWorkflowProvider is a WorkflowProvider queries a FHIR API to provide workflows.
type FhirApiWorkflowProvider struct {
	Client fhirclient.Client
}

// Provide returns the workflow for a given service and condition.
// It looks up the workflow through FHIR HealthcareServices in the FHIR API, searching for instances that match:
//   - Service code must be present in HealthcareService.category
//   - Condition code must be present in the HealthcareService.type
func (f FhirApiWorkflowProvider) Provide(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error) {
	if serviceCode.System == nil || serviceCode.Code == nil || conditionCode.System == nil || conditionCode.Code == nil {
		return nil, errors.New("serviceCode and conditionCode must have a system and code")
	}
	if err := f.searchHealthcareService(ctx, serviceCode, conditionCode); err != nil {
		return nil, err
	}
	questionnaireUrl := f.Client.Path("Questionnaire")
	questionnaireUrl.RawQuery = url.Values{
		"context": []string{
			"focus$" + *serviceCode.System + "|" + *serviceCode.Code,
			"focus$" + *conditionCode.System + "|" + *conditionCode.Code,
		},
	}.Encode()
	return &Workflow{
		Steps: []WorkflowStep{
			{
				QuestionnaireUrl: questionnaireUrl.String(),
			},
		},
	}, nil
}

func (f FhirApiWorkflowProvider) searchHealthcareService(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) error {
	queryParams := []fhirclient.Option{
		fhirclient.QueryParam("service-category", *serviceCode.System+"|"+*serviceCode.Code),
		fhirclient.QueryParam("service-type", *conditionCode.System+"|"+*conditionCode.Code),
	}
	var results fhir.Bundle
	if err := f.Client.ReadWithContext(ctx, "HealthcareService", &results, queryParams...); err != nil {
		return err
	}
	if len(results.Entry) == 0 {
		return ErrWorkflowNotFound
	}
	if len(results.Entry) > 2 {
		return errors.Join(ErrWorkflowNotFound, errors.New("multiple workflows found"))
	}
	return nil
}

func (f FhirApiWorkflowProvider) QuestionnaireLoader() QuestionnaireLoader {
	return FhirApiQuestionnaireLoader{
		client: f.Client,
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
