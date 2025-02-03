package taskengine

import (
	"context"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
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

	var questionnaireBundle fhir.Bundle
	if err := f.Client.Read("Questionnaire", &questionnaireBundle,
		fhirclient.QueryParam("context-type-value", *serviceCode.System+"|"+*serviceCode.Code),
		fhirclient.QueryParam("context-type-value", *conditionCode.System+"|"+*conditionCode.Code),
	); err != nil {
		return nil, err
	}
	// TODO: Might want to support multiple questionnaires in future
	if len(questionnaireBundle.Entry) != 1 {
		return nil, errors.Join(ErrWorkflowNotFound, fmt.Errorf("expected 1 questionnaire, got %d", len(questionnaireBundle.Entry)))
	}
	return &Workflow{
		Steps: []WorkflowStep{
			{
				QuestionnaireUrl: *questionnaireBundle.Entry[0].FullUrl,
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

var _ WorkflowProvider = &MemoryWorkflowProvider{}
var _ QuestionnaireLoader = &MemoryWorkflowProvider{}

// MemoryWorkflowProvider is a WorkflowProvider that uses in-memory FHIR resources to provide workflows.
// To use this provider, you must first load the resources using LoadBundle.
type MemoryWorkflowProvider struct {
	questionnaires     []fhir.Questionnaire
	healthcareServices []fhir.HealthcareService
}

// LoadBundle fetches the FHIR Bundle from the given URL and adds the contained Questionnaires and HealthcareServices to the provider.
// They can then be used to provide workflows.
func (e *MemoryWorkflowProvider) LoadBundle(ctx context.Context, bundleUrl string) error {
	var bundle fhir.Bundle
	parsedBundleUrl, err := url.Parse(bundleUrl)
	if err != nil {
		return err
	}
	client := fhirclient.New(parsedBundleUrl, http.DefaultClient, nil)
	if err := client.ReadWithContext(ctx, "", &bundle, fhirclient.AtUrl(parsedBundleUrl)); err != nil {
		return err
	}

	var questionnaires []fhir.Questionnaire
	if err := coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Questionnaire"), &questionnaires); err != nil {
		return fmt.Errorf("could not extract questionnaires from bundle: %w", err)
	}
	e.questionnaires = append(e.questionnaires, questionnaires...)

	var healthcareServices []fhir.HealthcareService
	if err := coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("HealthcareService"), &healthcareServices); err != nil {
		return fmt.Errorf("could not extract healthcare services from bundle: %w", err)
	}
	e.healthcareServices = append(e.healthcareServices, healthcareServices...)
	return nil
}

func (e *MemoryWorkflowProvider) Provide(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error) {
	// Mimicks Questionnaire and HealthcareService search like it's done in FhirApiWorkflowProvider, but just in-memory filtering.
	supported := false
	for _, healthcareService := range e.healthcareServices {
		if coolfhir.ConceptContainsCoding(serviceCode, healthcareService.Category...) && coolfhir.ConceptContainsCoding(conditionCode, healthcareService.Type...) {
			supported = true
			break
		}
	}
	if !supported {
		return nil, ErrWorkflowNotFound
	}
	for _, questionnaire := range e.questionnaires {
		matchesServiceCode := false
		matchesConditionCode := false
		for _, usageContext := range questionnaire.UseContext {
			if usageContext.ValueCodeableConcept == nil {
				continue
			}
			if coolfhir.ConceptContainsCoding(serviceCode, *usageContext.ValueCodeableConcept) {
				matchesServiceCode = true
			}
			if coolfhir.ConceptContainsCoding(conditionCode, *usageContext.ValueCodeableConcept) {
				matchesConditionCode = true
			}
		}
		if matchesServiceCode && matchesConditionCode {
			return &Workflow{
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "Questionnaire/" + *questionnaire.Id,
					},
				},
			}, nil
		}
	}
	return nil, ErrWorkflowNotFound
}

func (e *MemoryWorkflowProvider) Load(ctx context.Context, questionnaireUrl string) (*fhir.Questionnaire, error) {
	for _, questionnaire := range e.questionnaires {
		if "Questionnaire/"+*questionnaire.Id == questionnaireUrl {
			return &questionnaire, nil
		}
	}
	return nil, errors.New("questionnaire not found")
}

func (e *MemoryWorkflowProvider) QuestionnaireLoader() QuestionnaireLoader {
	return e
}

type Workflow struct {
	Steps []WorkflowStep
}

func (w Workflow) Start() WorkflowStep {
	return w.Steps[0]
}

func (w Workflow) Proceed(previousQuestionnaireRef string) (*WorkflowStep, error) {
	for i, step := range w.Steps {
		if strings.HasSuffix(step.QuestionnaireUrl, previousQuestionnaireRef) {
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
