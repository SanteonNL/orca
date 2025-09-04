package taskengine

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrWorkflowNotFound = errors.New("workflow not found")

var tracer = baseotel.Tracer("careplancontributor")

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
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("service.system", to.EmptyString(serviceCode.System)),
			attribute.String("service.code", to.EmptyString(serviceCode.Code)),
			attribute.String("condition.system", to.EmptyString(conditionCode.System)),
			attribute.String("condition.code", to.EmptyString(conditionCode.Code)),
		),
	)
	defer span.End()

	if serviceCode.System == nil || serviceCode.Code == nil || conditionCode.System == nil || conditionCode.Code == nil {
		err := errors.New("serviceCode and conditionCode must have a system and code")
		return nil, otel.Error(span, err)
	}

	if err := f.searchHealthcareService(ctx, serviceCode, conditionCode); err != nil {
		return nil, otel.Error(span, err)
	}

	var questionnaireBundle fhir.Bundle
	if err := f.Client.Read("Questionnaire", &questionnaireBundle,
		fhirclient.QueryParam("context-type-value", *serviceCode.System+"|"+*serviceCode.Code),
		fhirclient.QueryParam("context-type-value", *conditionCode.System+"|"+*conditionCode.Code),
	); err != nil {
		return nil, otel.Error(span, err)
	}
	// TODO: Might want to support multiple questionnaires in future
	if len(questionnaireBundle.Entry) != 1 {
		err := errors.Join(ErrWorkflowNotFound, fmt.Errorf("expected 1 questionnaire, got %d", len(questionnaireBundle.Entry)))
		return nil, otel.Error(span, err)
	}

	workflow := &Workflow{
		Steps: []WorkflowStep{
			{
				QuestionnaireUrl: *questionnaireBundle.Entry[0].FullUrl,
			},
		},
	}

	span.SetAttributes(
		attribute.Int("questionnaire.count", len(questionnaireBundle.Entry)),
		attribute.String("questionnaire.url", *questionnaireBundle.Entry[0].FullUrl),
		attribute.Int("workflow.steps", len(workflow.Steps)),
	)
	span.SetStatus(codes.Ok, "")
	return workflow, nil
}

func (f FhirApiWorkflowProvider) searchHealthcareService(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("service.system", to.EmptyString(serviceCode.System)),
			attribute.String("service.code", to.EmptyString(serviceCode.Code)),
			attribute.String("condition.system", to.EmptyString(conditionCode.System)),
			attribute.String("condition.code", to.EmptyString(conditionCode.Code)),
		),
	)
	defer span.End()

	queryParams := []fhirclient.Option{
		fhirclient.QueryParam("service-category", *serviceCode.System+"|"+*serviceCode.Code),
		fhirclient.QueryParam("service-type", *conditionCode.System+"|"+*conditionCode.Code),
	}
	var results fhir.Bundle
	if err := f.Client.ReadWithContext(ctx, "HealthcareService", &results, queryParams...); err != nil {
		return otel.Error(span, err)
	}

	span.SetAttributes(
		attribute.Int("healthcare_service.count", len(results.Entry)),
	)

	if len(results.Entry) == 0 {
		return otel.Error(span, ErrWorkflowNotFound, "no healthcare services found")
	}
	if len(results.Entry) > 2 {
		err := errors.Join(ErrWorkflowNotFound, errors.New("multiple workflows found"))
		return otel.Error(span, err)
	}

	span.SetStatus(codes.Ok, "")
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
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("bundle.url", bundleUrl),
		),
	)
	defer span.End()

	var bundle fhir.Bundle
	parsedBundleUrl, err := url.Parse(bundleUrl)
	if err != nil {
		return otel.Error(span, err)
	}

	client := fhirclient.New(parsedBundleUrl, otel.NewTracedHTTPClient("taskengine.LoadBundle"), coolfhir.Config())
	if err := client.ReadWithContext(ctx, "", &bundle, fhirclient.AtUrl(parsedBundleUrl)); err != nil {
		return otel.Error(span, err)
	}

	var questionnaires []fhir.Questionnaire
	if err := coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("Questionnaire"), &questionnaires); err != nil {
		err = fmt.Errorf("could not extract questionnaires from bundle: %w", err)
		return otel.Error(span, err)
	}
	e.questionnaires = append(e.questionnaires, questionnaires...)

	var healthcareServices []fhir.HealthcareService
	if err := coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("HealthcareService"), &healthcareServices); err != nil {
		err = fmt.Errorf("could not extract healthcare services from bundle: %w", err)
		return otel.Error(span, err)
	}
	e.healthcareServices = append(e.healthcareServices, healthcareServices...)

	span.SetAttributes(
		attribute.Int("bundle.total_entries", len(bundle.Entry)),
		attribute.Int("questionnaires.loaded", len(questionnaires)),
		attribute.Int("healthcare_services.loaded", len(healthcareServices)),
		attribute.Int("questionnaires.total", len(e.questionnaires)),
		attribute.Int("healthcare_services.total", len(e.healthcareServices)),
	)
	span.SetStatus(codes.Ok, "")
	return nil
}

func (e *MemoryWorkflowProvider) Provide(ctx context.Context, serviceCode fhir.Coding, conditionCode fhir.Coding) (*Workflow, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("service.system", to.EmptyString(serviceCode.System)),
			attribute.String("service.code", to.EmptyString(serviceCode.Code)),
			attribute.String("condition.system", to.EmptyString(conditionCode.System)),
			attribute.String("condition.code", to.EmptyString(conditionCode.Code)),
			attribute.String("provider.type", "memory"),
		),
	)
	defer span.End()

	// Mimicks Questionnaire and HealthcareService search like it's done in FhirApiWorkflowProvider, but just in-memory filtering.
	supported := false
	for _, healthcareService := range e.healthcareServices {
		if coolfhir.ConceptContainsCoding(serviceCode, healthcareService.Category...) && coolfhir.ConceptContainsCoding(conditionCode, healthcareService.Type...) {
			supported = true
			break
		}
	}

	span.SetAttributes(
		attribute.Int("healthcare_services.total", len(e.healthcareServices)),
		attribute.Bool("workflow.supported", supported),
	)

	if !supported {
		return nil, otel.Error(span, ErrWorkflowNotFound, "Workflow not supported by any healthcare service")
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
			workflow := &Workflow{
				Steps: []WorkflowStep{
					{
						QuestionnaireUrl: "Questionnaire/" + *questionnaire.Id,
					},
				},
			}

			span.SetAttributes(
				attribute.Int("questionnaires.total", len(e.questionnaires)),
				attribute.String("questionnaire.id", to.EmptyString(questionnaire.Id)),
				attribute.String("questionnaire.url", "Questionnaire/"+to.EmptyString(questionnaire.Id)),
				attribute.Int("workflow.steps", len(workflow.Steps)),
			)
			span.SetStatus(codes.Ok, "")
			return workflow, nil
		}
	}

	span.SetAttributes(
		attribute.Int("questionnaires.total", len(e.questionnaires)),
	)
	return nil, otel.Error(span, ErrWorkflowNotFound, "No matching questionnaire found")
}

func (e *MemoryWorkflowProvider) Load(ctx context.Context, questionnaireUrl string) (*fhir.Questionnaire, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("questionnaire.url", questionnaireUrl),
			attribute.String("provider.type", "memory"),
		),
	)
	defer span.End()

	for i, questionnaire := range e.questionnaires {
		if "Questionnaire/"+*questionnaire.Id == questionnaireUrl {
			span.SetAttributes(
				attribute.String("questionnaire.id", to.EmptyString(questionnaire.Id)),
				attribute.Int("questionnaires.searched", i+1),
				attribute.Int("questionnaires.total", len(e.questionnaires)),
			)
			span.SetStatus(codes.Ok, "")
			return &questionnaire, nil
		}
	}

	span.SetAttributes(
		attribute.Int("questionnaires.searched", len(e.questionnaires)),
		attribute.Int("questionnaires.total", len(e.questionnaires)),
	)
	return nil, otel.Error(span, errors.New("questionnaire not found"))
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
