package main

import (
	"context"
	"fmt"
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

func setupOrchestrator(t *testing.T, dockerNetworkName string, containerName string, tenant string, cpsEnabled bool, fhirStoreURL string, questionnaireFhirStoreUrl string) *url.URL {
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
		Env: map[string]string{
			"ORCA_LOGLEVEL":                                 "debug",
			"ORCA_PUBLIC_URL":                               "http://" + containerName + ":8080",
			"ORCA_NUTS_API_URL":                             "http://nutsnode:8081",
			"ORCA_NUTS_PUBLIC_URL":                          "http://nutsnode:8080",
			"ORCA_TENANT_" + tenant + "_NUTS_SUBJECT":       tenant,
			"ORCA_TENANT_" + tenant + "_CPS_FHIR_URL":       fhirStoreURL,
			"ORCA_TENANT_" + tenant + "_DEMO_FHIR_URL":      fhirStoreURL,
			"ORCA_TENANT_" + tenant + "_TASKENGINE_ENABLED": "true",
			"ORCA_NUTS_DISCOVERYSERVICE":                    "dev:HomeMonitoring2024",
			"ORCA_CAREPLANSERVICE_ENABLED":                  strconv.FormatBool(cpsEnabled),
			// HAPI FHIR can only store Questionnaires in the default partition.
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_URL": questionnaireFhirStoreUrl,
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS": "file:///config/fhir/healthcareservices.json,file:///config/fhir/questionnaires.json",
			"ORCA_CAREPLANCONTRIBUTOR_ENABLED":                          "true",
			"ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN":                "valid",
			"ORCA_CAREPLANCONTRIBUTOR_APPLAUNCH_DEMO_ENABLED":           "true",
			"ORCA_STRICTMODE": "false",

			// OpenTelemetry Configuration
			"ORCA_OPENTELEMETRY_ENABLED":                "true",
			"ORCA_OPENTELEMETRY_SERVICE_NAME":           "orca-orchestrator-" + nutsSubject,
			"ORCA_OPENTELEMETRY_SERVICE_VERSION":        "e2e-test",
			"ORCA_OPENTELEMETRY_EXPORTER_TYPE":          "otlp",
			"ORCA_OPENTELEMETRY_EXPORTER_OTLP_ENDPOINT": "otel-collector:4318",
			"ORCA_OPENTELEMETRY_EXPORTER_OTLP_INSECURE": "true",

			// Explicitly set standard OTEL env vars to prevent conflicts
			"OTEL_EXPORTER_OTLP_ENDPOINT":        "",
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "",
			"OTEL_EXPORTER_OTLP_INSECURE":        "",
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

// setupOTELCollector sets up an OpenTelemetry Collector container to collect traces
func setupOTELCollector(t *testing.T, dockerNetworkName string) {
	println("Starting OTEL Collector for trace collection...")
	ctx := context.Background()

	// Create OTEL Collector configuration - simplified to avoid file system issues
	collectorConfig := `
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:

exporters:
  debug:
    verbosity: detailed
    sampling_initial: 2
    sampling_thereafter: 50

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`

	// Create log consumer to capture traces
	logConsumer := NewOTELCollectorLogConsumer()

	req := testcontainers.ContainerRequest{
		Image:        "otel/opentelemetry-collector:latest",
		Name:         "otel-collector",
		ExposedPorts: []string{"4317/tcp", "4318/tcp"},
		Networks:     []string{dockerNetworkName},
		Files: []testcontainers.ContainerFile{
			{
				Reader:            strings.NewReader(collectorConfig),
				ContainerFilePath: "/etc/otel-collector-config.yaml",
				FileMode:          0644,
			},
		},
		Cmd: []string{"--config=/etc/otel-collector-config.yaml"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("4317/tcp").WithStartupTimeout(30*time.Second),
			wait.ForListeningPort("4318/tcp").WithStartupTimeout(30*time.Second),
		),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{logConsumer},
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		// Extract traces before container shutdown
		extractOTELCollectorTraces(t, logConsumer)
		_ = container.Terminate(ctx)
	})

	println("OTEL Collector started and ready to receive traces")
}

// extractOTELCollectorTraces saves the captured traces to a file
func extractOTELCollectorTraces(t *testing.T, logConsumer *OTELCollectorLogConsumer) {
	t.Log("Extracting traces from OTEL Collector")

	// Wait a moment for final traces to be processed
	time.Sleep(2 * time.Second)

	// Save traces to file
	tracesFile := "otel_traces.txt"
	err := logConsumer.SaveTraces(tracesFile)
	if err != nil {
		t.Logf("Failed to save traces: %v", err)
		return
	}

	t.Logf("Traces exported to: %s", tracesFile)
}

// NewOTELCollectorLogConsumer creates a log consumer that captures and saves OTEL traces
func NewOTELCollectorLogConsumer() *OTELCollectorLogConsumer {
	return &OTELCollectorLogConsumer{
		traces: make([]string, 0),
	}
}

type OTELCollectorLogConsumer struct {
	traces []string
}

func (o *OTELCollectorLogConsumer) Accept(l testcontainers.Log) {
	logLine := string(l.Content)
	logLine = strings.TrimSpace(logLine)

	if logLine == "" {
		return
	}

	o.traces = append(o.traces, logLine)
}

func (o *OTELCollectorLogConsumer) SaveTraces(filename string) error {
	if len(o.traces) == 0 {
		return fmt.Errorf("no traces captured")
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write traces as simple text file
	for _, trace := range o.traces {
		_, err := file.WriteString(trace + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}
