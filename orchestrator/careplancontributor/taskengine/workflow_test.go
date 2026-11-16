package taskengine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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
			ctx, spans := traceTest(t)
			_, err := provider.Provide(ctx, serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
			requireWorkflowNotFoundTraced(t, spans())
		})
		t.Run("condition not supported", func(t *testing.T) {
			conditionCode := fhir.Coding{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("1234"),
			}
			ctx, spans := traceTest(t)
			_, err := provider.Provide(ctx, serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
			requireWorkflowNotFoundTraced(t, spans())
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

var testSpanExporter = tracetest.NewInMemoryExporter()

// TestMain installs a TracerProvider that records spans in memory. It's installed once for the whole package,
// since the package-level tracer binds to the first global TracerProvider that's set.
func TestMain(m *testing.M) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(testSpanExporter))
	baseotel.SetTracerProvider(tp)
	code := m.Run()
	_ = tp.Shutdown(context.Background())
	os.Exit(code)
}

// traceTest starts a root span for the calling test and returns a context to pass to the code under test,
// and a function that ends the root span and returns the other spans of that trace (only).
// Filtering by trace keeps tests isolated, even when run in parallel.
func traceTest(t *testing.T) (context.Context, func() tracetest.SpanStubs) {
	ctx, root := baseotel.Tracer("test").Start(context.Background(), t.Name())
	return ctx, func() tracetest.SpanStubs {
		root.End()
		var result tracetest.SpanStubs
		for _, span := range testSpanExporter.GetSpans() {
			if span.SpanContext.TraceID() == root.SpanContext().TraceID() && span.SpanContext.SpanID() != root.SpanContext().SpanID() {
				result = append(result, span)
			}
		}
		return result
	}
}

// requireWorkflowNotFoundTraced asserts that a "workflow not found" outcome is recorded in tracing, but not as an error/exception,
// since callers try multiple service/condition combinations and a single miss is expected (DEV-3401).
func requireWorkflowNotFoundTraced(t *testing.T, spans tracetest.SpanStubs) {
	t.Helper()
	require.NotEmpty(t, spans)
	for _, span := range spans {
		require.NotEqualf(t, codes.Error, span.Status.Code, "span %s has error status: %s", span.Name, span.Status.Description)
		require.Falsef(t, hasEvent(span, "exception"), "span %s recorded an exception", span.Name)
	}
	provideSpan := findSpan(t, spans, ".Provide")
	require.Truef(t, hasEvent(provideSpan, "workflow not found"), "span %s has no 'workflow not found' event", provideSpan.Name)
	require.Contains(t, provideSpan.Attributes, attribute.Bool("workflow.found", false))
}

// requireErrorTraced asserts that the Provide span is marked as failed.
func requireErrorTraced(t *testing.T, spans tracetest.SpanStubs) {
	t.Helper()
	provideSpan := findSpan(t, spans, ".Provide")
	require.Equal(t, codes.Error, provideSpan.Status.Code)
	require.True(t, hasEvent(provideSpan, "exception"))
}

func findSpan(t *testing.T, spans tracetest.SpanStubs, nameSuffix string) tracetest.SpanStub {
	t.Helper()
	for _, span := range spans {
		if strings.HasSuffix(span.Name, nameSuffix) {
			return span
		}
	}
	require.Failf(t, "span not found", "no span with name suffix %s", nameSuffix)
	return tracetest.SpanStub{}
}

func hasEvent(span tracetest.SpanStub, name string) bool {
	for _, event := range span.Events {
		if event.Name == name {
			return true
		}
	}
	return false
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
	twoHealthcareServicesBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"HealthcareService","id":"1"}},{"resource":{"resourceType":"HealthcareService","id":"2"}}]}`
	threeHealthcareServicesBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"resource":{"resourceType":"HealthcareService","id":"1"}},{"resource":{"resourceType":"HealthcareService","id":"2"}},{"resource":{"resourceType":"HealthcareService","id":"3"}}]}`
	questionnaireBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"fullUrl":"Questionnaire/1","resource":{"resourceType":"Questionnaire","id":"1","status":"active"}}]}`
	twoQuestionnairesBundle := `{"resourceType":"Bundle","type":"searchset","entry":[{"fullUrl":"Questionnaire/1","resource":{"resourceType":"Questionnaire","id":"1","status":"active"}},{"fullUrl":"Questionnaire/2","resource":{"resourceType":"Questionnaire","id":"2","status":"active"}}]}`

	t.Run("ok", func(t *testing.T) {
		provider := newProvider(t, healthcareServiceBundle, questionnaireBundle)
		workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
		require.NoError(t, err)
		require.Equal(t, "Questionnaire/1", workflow.Start().QuestionnaireUrl)
	})
	t.Run("no healthcare service found", func(t *testing.T) {
		ctx, spans := traceTest(t)
		provider := newProvider(t, emptyBundle, emptyBundle)
		_, err := provider.Provide(ctx, serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrWorkflowNotFound)
		requireWorkflowNotFoundTraced(t, spans())
	})
	t.Run("no questionnaire found", func(t *testing.T) {
		ctx, spans := traceTest(t)
		provider := newProvider(t, healthcareServiceBundle, emptyBundle)
		_, err := provider.Provide(ctx, serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrWorkflowNotFound)
		requireWorkflowNotFoundTraced(t, spans())
	})
	t.Run("2 healthcare services found, resolves through single questionnaire", func(t *testing.T) {
		provider := newProvider(t, twoHealthcareServicesBundle, questionnaireBundle)
		workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
		require.NoError(t, err)
		require.Equal(t, "Questionnaire/1", workflow.Start().QuestionnaireUrl)
	})
	t.Run("more than 2 healthcare services found", func(t *testing.T) {
		ctx, spans := traceTest(t)
		provider := newProvider(t, threeHealthcareServicesBundle, questionnaireBundle)
		_, err := provider.Provide(ctx, serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrMultipleWorkflows)
		require.NotErrorIs(t, err, ErrWorkflowNotFound)
		requireErrorTraced(t, spans())
	})
	t.Run("multiple questionnaires found", func(t *testing.T) {
		ctx, spans := traceTest(t)
		provider := newProvider(t, healthcareServiceBundle, twoQuestionnairesBundle)
		_, err := provider.Provide(ctx, serviceCode, conditionCode)
		require.ErrorIs(t, err, ErrMultipleWorkflows)
		require.NotErrorIs(t, err, ErrWorkflowNotFound)
		requireErrorTraced(t, spans())
	})
}
