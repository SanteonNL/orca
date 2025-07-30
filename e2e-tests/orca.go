package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupJaeger sets up a Jaeger v2 container to collect OpenTelemetry traces
func setupJaeger(t *testing.T, dockerNetworkName string) *url.URL {
	println("Starting Jaeger v2 for OpenTelemetry traces...")
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "jaegertracing/jaeger:2.0.0", // Updated to Jaeger v2
		Name:         "jaeger",
		ExposedPorts: []string{"16686/tcp", "4317/tcp", "4318/tcp"}, // Removed deprecated ports
		Networks:     []string{dockerNetworkName},
		Env: map[string]string{
			// Jaeger v2 uses different environment variable names
			"JAEGER_HTTP_SERVER_HOST_PORT":  ":16686",
			"COLLECTOR_OTLP_GRPC_HOST_PORT": ":4317",
			"COLLECTOR_OTLP_HTTP_HOST_PORT": ":4318",
		},
		WaitingFor: wait.ForHTTP("/").WithPort("16686/tcp"),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = container.Terminate(ctx)
	})

	// Get the exposed port for Jaeger UI
	mappedPort, err := container.MappedPort(ctx, "16686")
	require.NoError(t, err)

	jaegerUIURL := &url.URL{
		Scheme: "http",
		Host:   "localhost:" + mappedPort.Port(),
	}

	println("Jaeger UI available at:", jaegerUIURL.String())
	return jaegerUIURL
}

func setupOrchestrator(t *testing.T, dockerNetworkName string, containerName string, nutsSubject string, cpsEnabled bool, fhirStoreURL string, questionnaireFhirStoreUrl string) *url.URL {
	image := os.Getenv("ORCHESTRATOR_IMAGE")
	pullImage := false
	if image == "" {
		image = "ghcr.io/santeonnl/orca_orchestrator:main"
		pullImage = true
	}
	println("Starting ORCA Orchestrator with OpenTelemetry enabled...")
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:           image,
		Name:            containerName,
		ExposedPorts:    []string{"8080/tcp"},
		Networks:        []string{dockerNetworkName},
		AlwaysPullImage: pullImage,
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{NewFilteredLogConsumer(containerName)},
		},
		Env: map[string]string{
			"ORCA_LOGLEVEL":                     "debug",
			"ORCA_PUBLIC_URL":                   "http://" + containerName + ":8080",
			"ORCA_NUTS_API_URL":                 "http://nutsnode:8081",
			"ORCA_NUTS_PUBLIC_URL":              "http://nutsnode:8080",
			"ORCA_NUTS_SUBJECT":                 nutsSubject,
			"ORCA_NUTS_DISCOVERYSERVICE":        "dev:HomeMonitoring2024",
			"ORCA_CAREPLANSERVICE_ENABLED":      strconv.FormatBool(cpsEnabled),
			"ORCA_CAREPLANSERVICE_FHIR_URL":     fhirStoreURL,
			"ORCA_CAREPLANCONTRIBUTOR_FHIR_URL": fhirStoreURL,
			// HAPI FHIR can only store Questionnaires in the default partition.
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_URL": questionnaireFhirStoreUrl,
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS": "file:///config/fhir/healthcareservices.json,file:///config/fhir/questionnaires.json",
			"ORCA_CAREPLANCONTRIBUTOR_ENABLED":                          "true",
			"ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN":                "valid",
			"ORCA_STRICTMODE": "false",

			// OpenTelemetry Configuration
			"ORCA_OPENTELEMETRY_ENABLED":                "true",
			"ORCA_OPENTELEMETRY_SERVICE_NAME":           "orca-orchestrator-" + nutsSubject,
			"ORCA_OPENTELEMETRY_SERVICE_VERSION":        "e2e-test",
			"ORCA_OPENTELEMETRY_EXPORTER_TYPE":          "otlp",
			"ORCA_OPENTELEMETRY_EXPORTER_OTLP_ENDPOINT": "http://jaeger:4318",
		},
		Files: []testcontainers.ContainerFile{
			// Questionnaire and HealthcareService bundles required by Task Filler Engine
			{
				HostFilePath:      "../orchestrator/careplancontributor/taskengine/testdata/healthcareservice-bundle.json",
				ContainerFilePath: "/config/fhir/healthcareservices.json",
				FileMode:          0444,
			},
			{
				HostFilePath:      "../orchestrator/careplancontributor/taskengine/testdata/questionnaire-bundle.json",
				ContainerFilePath: "/config/fhir/questionnaires.json",
				FileMode:          0444,
			},
		},
		WaitingFor: wait.ForHTTP("/health"),
	}
	if questionnaireFhirStoreUrl == "" {
		delete(req.Env, "ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS")
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			panic(err)
		}
	})
	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(t, err)
	r, _ := url.Parse(endpoint)
	return r
}

// extractJaegerTraces exports all traces from Jaeger and saves them to a file
func extractJaegerTraces(t *testing.T, jaegerUIURL *url.URL) {
	t.Log("📊 Extracting traces from Jaeger before shutdown...")

	// Wait a moment for traces to be fully processed
	time.Sleep(2 * time.Second)

	// Get list of services
	servicesURL := jaegerUIURL.JoinPath("/api/services")
	resp, err := http.Get(servicesURL.String())
	if err != nil {
		t.Logf("❌ Failed to get services from Jaeger: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Logf("❌ Jaeger services API returned status %d", resp.StatusCode)
		return
	}

	var servicesResp struct {
		Data []string `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&servicesResp); err != nil {
		t.Logf("❌ Failed to decode services response: %v", err)
		return
	}

	t.Logf("🔍 Found %d services in Jaeger", len(servicesResp.Data))

	// Extract traces for each service
	allTraces := make(map[string]interface{})
	for _, service := range servicesResp.Data {
		traces := extractTracesForService(t, jaegerUIURL, service)
		if len(traces) > 0 {
			allTraces[service] = traces
			t.Logf("✅ Extracted %d traces for service: %s", len(traces), service)
		}
	}

	if len(allTraces) == 0 {
		t.Log("⚠️  No traces found in Jaeger")
		return
	}

	// Save traces to file
	tracesFile := "jaeger_traces.json"
	file, err := os.Create(tracesFile)
	if err != nil {
		t.Logf("❌ Failed to create traces file: %v", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(allTraces); err != nil {
		t.Logf("❌ Failed to write traces to file: %v", err)
		return
	}

	t.Logf("💾 Traces exported to: %s", tracesFile)

	// Also print a summary to console
	printTraceSummary(t, allTraces)
}

func extractTracesForService(t *testing.T, jaegerUIURL *url.URL, service string) []interface{} {
	// Look back 1 hour to catch all traces from the test
	endTime := time.Now().UnixMicro()
	startTime := time.Now().Add(-1 * time.Hour).UnixMicro()

	tracesURL := jaegerUIURL.JoinPath("/api/traces")
	req, err := http.NewRequest("GET", tracesURL.String(), nil)
	if err != nil {
		return nil
	}

	q := req.URL.Query()
	q.Set("service", service)
	q.Set("start", fmt.Sprintf("%d", startTime))
	q.Set("end", fmt.Sprintf("%d", endTime))
	q.Set("limit", "100") // Increase limit to catch more traces
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var tracesResp struct {
		Data []interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tracesResp); err != nil {
		return nil
	}

	return tracesResp.Data
}

func printTraceSummary(t *testing.T, allTraces map[string]interface{}) {
	t.Log("🎯 TRACE SUMMARY:")
	t.Log("================")

	for service, traces := range allTraces {
		if traceList, ok := traces.([]interface{}); ok {
			t.Logf("📋 Service: %s (%d traces)", service, len(traceList))

			// Print first few trace operations for preview
			for i, trace := range traceList {
				if i >= 3 { // Limit to first 3 traces per service
					t.Logf("    ... and %d more traces", len(traceList)-3)
					break
				}

				if traceMap, ok := trace.(map[string]interface{}); ok {
					if spans, ok := traceMap["spans"].([]interface{}); ok && len(spans) > 0 {
						if span, ok := spans[0].(map[string]interface{}); ok {
							if operationName, ok := span["operationName"].(string); ok {
								t.Logf("    🔸 %s", operationName)
							}
						}
					}
				}
			}
		}
	}
	t.Log("================")
}

// createLogFilter helps identify OTEL vs other logs
type LogFilter struct {
	OTELPatterns []string
	ServiceNames []string
}

func NewLogFilter() *LogFilter {
	return &LogFilter{
		OTELPatterns: []string{
			// OpenTelemetry specific
			"otel", "opentelemetry", "otlp",
			// Tracing terms
			"tracing", "span", "trace", "tracer",
			// Jaeger specific
			"jaeger", "jaeger-client",
			// ORCA OTEL config
			"ORCA_OPENTELEMETRY",
			// Common span/trace operations
			"span.start", "span.end", "span.finish",
			"trace.start", "trace.end",
			// Instrumentation
			"instrumentation", "telemetry",
			// Exporters
			"exporter", "collector",
			// Sampling
			"sampler", "sampling",
		},
		ServiceNames: []string{
			"orca-orchestrator-clinic",
			"orca-orchestrator-hospital",
		},
	}
}

func (lf *LogFilter) IsOTELLog(logLine string) bool {
	logLower := strings.ToLower(logLine)

	// Check for OTEL patterns
	for _, pattern := range lf.OTELPatterns {
		if strings.Contains(logLower, strings.ToLower(pattern)) {
			return true
		}
	}

	// Check for service names in tracing context
	for _, service := range lf.ServiceNames {
		if strings.Contains(logLower, strings.ToLower(service)) {
			return true
		}
	}

	return false
}
