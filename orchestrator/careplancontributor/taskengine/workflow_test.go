package taskengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestMemoryFHIRResourcesWorkflowProvider_Provide(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.String(), "healthcareservice") {
			data, err := os.ReadFile("testdata/healthcareservice-bundle.json")
			if err != nil {
				panic(err)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(data)
		} else if strings.Contains(request.URL.String(), "questionnaire") {
			data, err := os.ReadFile("testdata/questionnaire-bundle.json")
			if err != nil {
				panic(err)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(data)
		} else {
			writer.WriteHeader(http.StatusNotFound)
		}
	}))

	provider := &MemoryWorkflowProvider{}
	require.NoError(t, provider.LoadBundle(context.Background(), httpServer.URL+"/healthcareservice"))
	require.NoError(t, provider.LoadBundle(context.Background(), httpServer.URL+"/questionnaire"))

	serviceCode := fhir.Coding{
		System: to.Ptr("http://snomed.info/sct"),
		Code:   to.Ptr("719858009"),
	}
	conditionCode := fhir.Coding{
		System: to.Ptr("http://snomed.info/sct"),
		Code:   to.Ptr("84114007"),
	}

	t.Run("provide workflow", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.NoError(t, err)
			require.NotNil(t, workflow)

			t.Run("proceed, no more steps", func(t *testing.T) {
				step := workflow.Start()
				nextStep, err := workflow.Proceed(step.QuestionnaireUrl)
				require.NoError(t, err)
				require.Nil(t, nextStep)
			})
		})
		t.Run("service not supported", func(t *testing.T) {
			serviceCode := fhir.Coding{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("134"),
			}
			tp, exporter := setupSpanRecorder(t)
			_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
			requireNoErrorSpans(t, tp, exporter)
		})
		t.Run("condition not supported", func(t *testing.T) {
			conditionCode := fhir.Coding{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("1234"),
			}
			tp, exporter := setupSpanRecorder(t)
			_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
			requireNoErrorSpans(t, tp, exporter)
		})
	})
	t.Run("load questionnaire", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.NoError(t, err)
			require.NotNil(t, workflow)

			questionnaire, err := provider.QuestionnaireLoader().Load(context.Background(), workflow.Start().QuestionnaireUrl)
			require.NoError(t, err)
			require.NotNil(t, questionnaire)
		})
	})
}

var (
	spanRecorderOnce     sync.Once
	spanRecorderProvider *sdktrace.TracerProvider
	spanRecorderExporter *tracetest.InMemoryExporter
)

// setupSpanRecorder returns an in-memory span exporter, cleared for the calling test.
// The package-level tracer binds to the first global TracerProvider that's set, so a single provider is shared by all tests in this package.
func setupSpanRecorder(t *testing.T) (*sdktrace.TracerProvider, *tracetest.InMemoryExporter) {
	spanRecorderOnce.Do(func() {
		spanRecorderExporter = tracetest.NewInMemoryExporter()
		spanRecorderProvider = sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanRecorderExporter))
		baseotel.SetTracerProvider(spanRecorderProvider)
	})
	spanRecorderExporter.Reset()
	t.Cleanup(spanRecorderExporter.Reset)
	return spanRecorderProvider, spanRecorderExporter
}

// requireNoErrorSpans asserts that a "workflow not found" outcome is not reported as an error/exception in tracing,
// since callers try multiple service/condition combinations and a single miss is expected (DEV-3401).
func requireNoErrorSpans(t *testing.T, tp *sdktrace.TracerProvider, exporter *tracetest.InMemoryExporter) {
	t.Helper()
	require.NoError(t, tp.ForceFlush(context.Background()))
	spans := exporter.GetSpans()
	require.NotEmpty(t, spans)
	for _, span := range spans {
		require.NotEqualf(t, codes.Error, span.Status.Code, "span %s has error status: %s", span.Name, span.Status.Description)
		for _, event := range span.Events {
			require.NotEqualf(t, "exception", event.Name, "span %s recorded an exception", span.Name)
		}
	}
}

func TestFhirApiWorkflowProvider_Provide(t *testing.T) {
	emptyBundle := `{"resourceType":"Bundle","type":"searchset","entry":[]}`
	newProvider := func(t *testing.T, healthcareServices string, questionnaires string) FhirApiWorkflowProvider {
		httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/fhir+json")
			switch {
			case strings.HasPrefix(request.URL.Path, "/HealthcareService"):
				_, _ = writer.Write([]byte(healthcareServices))
			case strings.HasPrefix(request.URL.Path, "/Questionnaire"):
				_, _ = writer.Write([]byte(questionnaires))
			default:
				writer.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(httpServer.Close)
		baseURL, _ := url.Parse(httpServer.URL)
		return FhirApiWorkflowProvider{Client: fhirclient.New(baseURL, httpServer.Client(), coolfhir.Config())}
	}
	serviceCode := fhir.Coding{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("719858009")}
	conditionCode := fhir.Coding{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("84114007")}
	healthcareServiceBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"HealthcareService","id":"1"}}]}`

	t.Run("ok", func(t *testing.T) {
		questionnaireBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"fullUrl":"Questionnaire/1","resource":{"resourceType":"Questionnaire","id":"1","status":"active"}}]}`
		provider := newProvider(t, healthcareServiceBundle, questionnaireBundle)
		workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
		require.NoError(t, err)
		require.Equal(t, "Questionnaire/1", workflow.Start().QuestionnaireUrl)
	})
	t.Run("no healthcare service found", func(t *testing.T) {
		tp, exporter := setupSpanRecorder(t)
		provider := newProvider(t, emptyBundle, emptyBundle)
		_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrWorkflowNotFound)
		requireNoErrorSpans(t, tp, exporter)
	})
	t.Run("no questionnaire found", func(t *testing.T) {
		tp, exporter := setupSpanRecorder(t)
		provider := newProvider(t, healthcareServiceBundle, emptyBundle)
		_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrWorkflowNotFound)
		requireNoErrorSpans(t, tp, exporter)
	})
}
