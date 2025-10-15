package otel

import (
	"context"
	"fmt"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"os"
	"strings"
	"time"
)

// Config holds the OpenTelemetry configuration
type Config struct {
	// Enabled controls whether OpenTelemetry is enabled
	Enabled bool `koanf:"enabled"`
	// ServiceName is the name of the service for tracing
	ServiceName string `koanf:"service_name"`
	// ServiceVersion is the version of the service
	ServiceVersion string `koanf:"service_version"`
	// ResourceAttributes additional resource attributes
	ResourceAttributes map[string]string `koanf:"resource_attributes"`
	// Exporter configuration
	Exporter ExporterConfig `koanf:"exporter"`
}

type ExporterConfig struct {
	// Type of exporter: "otlp", "stdout", or "none"
	Type string `koanf:"type"`
	// Protocol for OTLP exporter: "grpc" or "http"
	Protocol string `koanf:"protocol"`
	// OTLP exporter configuration (when type is "otlp")
	OTLP OTLPConfig `koanf:"otlp"`
}

type OTLPConfig struct {
	// Endpoint for the OTLP exporter (e.g., "localhost:4317" for gRPC insecure, "https://endpoint.com" for secure)
	Endpoint string `koanf:"endpoint"`
	// MetricEndpoint for OTLP metrics (optional override)
	MetricEndpoint string `koanf:"metric_endpoint"`
	// LoggingEndpoint for OTLP logging (optional override)
	LoggingEndpoint string `koanf:"logging_endpoint"`
	// Headers to send with OTLP requests
	Headers map[string]string `koanf:"headers"`
	// Timeout for OTLP requests
	Timeout time.Duration `koanf:"timeout"`
	// Insecure controls whether to use insecure gRPC connection
	Insecure bool `koanf:"insecure"`
}

// DefaultConfig returns a default OTEL configuration
func DefaultConfig() Config {
	// Default values
	endpoint := "localhost:4317"
	protocol := "grpc"
	insecure := true
	serviceName := "orca-orchestrator"
	var metricEndpoint, loggingEndpoint string
	resourceAttributes := make(map[string]string)

	// Read OTEL_SERVICE_NAME
	if envServiceName := os.Getenv("OTEL_SERVICE_NAME"); envServiceName != "" {
		serviceName = envServiceName
	}

	// Read OTEL_EXPORTER_OTLP_PROTOCOL
	if envProtocol := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); envProtocol != "" {
		protocol = envProtocol
	}

	// Read OTEL_EXPORTER_OTLP_ENDPOINT
	if envEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); envEndpoint != "" {
		endpoint = envEndpoint
		// If the original endpoint had https://, we should use secure connection
		insecure = !strings.HasPrefix(envEndpoint, "https://")

		// For gRPC, remove scheme prefix
		if protocol == "grpc" {
			endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
			// Remove trailing slash if present
			endpoint = strings.TrimSuffix(endpoint, "/")
		}
	}

	// Read Azure Container App specific endpoints
	if envMetricEndpoint := os.Getenv("CONTAINERAPP_OTEL_METRIC_GRPC_ENDPOINT"); envMetricEndpoint != "" {
		metricEndpoint = envMetricEndpoint
	}

	if envLoggingEndpoint := os.Getenv("CONTAINERAPP_OTEL_LOGGING_GRPC_ENDPOINT"); envLoggingEndpoint != "" {
		loggingEndpoint = envLoggingEndpoint
	}

	// Parse OTEL_RESOURCE_ATTRIBUTES
	if envResourceAttrs := os.Getenv("OTEL_RESOURCE_ATTRIBUTES"); envResourceAttrs != "" {
		pairs := strings.Split(envResourceAttrs, ",")
		for _, pair := range pairs {
			if kv := strings.SplitN(strings.TrimSpace(pair), "=", 2); len(kv) == 2 {
				resourceAttributes[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	return Config{
		Enabled:            true,
		ServiceName:        serviceName,
		ServiceVersion:     "1.0.0",
		ResourceAttributes: resourceAttributes,
		Exporter: ExporterConfig{
			Type:     "otlp",
			Protocol: protocol,
			OTLP: OTLPConfig{
				Insecure:        insecure,
				Endpoint:        endpoint,
				MetricEndpoint:  metricEndpoint,
				LoggingEndpoint: loggingEndpoint,
				Timeout:         10 * time.Second,
			},
		},
	}
}

// Validate validates the OTEL configuration
func (c Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	if c.ServiceName == "" {
		return fmt.Errorf("service name is required when OpenTelemetry is enabled")
	}

	switch c.Exporter.Type {
	case "otlp":
		if c.Exporter.OTLP.Endpoint == "" {
			return fmt.Errorf("OTLP endpoint is required when using OTLP exporter")
		}
	case "stdout", "none":
		// Valid types, no additional validation needed
	default:
		return fmt.Errorf("unsupported exporter type: %s (supported: otlp, stdout, none)", c.Exporter.Type)
	}

	return nil
}

// TracerProvider holds the global tracer provider and cleanup function
type TracerProvider struct {
	provider *trace.TracerProvider
	cleanup  func(context.Context) error
}

// Initialize sets up OpenTelemetry based on the configuration
func Initialize(ctx context.Context, config Config) (*TracerProvider, error) {
	if !config.Enabled {
		// Set up a no-op tracer provider
		noopProvider := trace.NewTracerProvider()
		baseotel.SetTracerProvider(noopProvider)
		return &TracerProvider{
			provider: noopProvider,
			cleanup:  func(context.Context) error { return nil },
		}, nil
	}

	// Create resource attributes slice
	resourceOpts := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	}

	// Add additional resource attributes from environment
	if len(config.ResourceAttributes) > 0 {
		var attrs []attribute.KeyValue
		for key, value := range config.ResourceAttributes {
			attrs = append(attrs, attribute.String(key, value))
		}
		resourceOpts = append(resourceOpts, resource.WithAttributes(attrs...))
	}

	// Create resource with service information
	res, err := resource.New(ctx, resourceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter trace.SpanExporter
	switch config.Exporter.Type {
	case "otlp":
		// Choose exporter based on protocol
		switch config.Exporter.Protocol {
		case "grpc":
			opts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(config.Exporter.OTLP.Endpoint),
				otlptracegrpc.WithTimeout(config.Exporter.OTLP.Timeout),
			}

			// Add headers if configured
			if len(config.Exporter.OTLP.Headers) > 0 {
				opts = append(opts, otlptracegrpc.WithHeaders(config.Exporter.OTLP.Headers))
			}

			// Use insecure gRPC connection if configured
			if config.Exporter.OTLP.Insecure {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}

			exporter, err = otlptracegrpc.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
			}

		default:
			return nil, fmt.Errorf("unsupported OTLP protocol: %s (supported: grpc)", config.Exporter.Protocol)
		}

	case "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
		}
	case "none":
		// No exporter, traces will be collected but not exported
		exporter = nil
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", config.Exporter.Type)
	}

	// Create tracer provider
	var opts []trace.TracerProviderOption
	opts = append(opts, trace.WithResource(res))

	if exporter != nil {
		opts = append(opts, trace.WithBatcher(exporter))
	}

	tp := trace.NewTracerProvider(opts...)

	// Set global tracer provider
	baseotel.SetTracerProvider(tp)

	// Set global propagator to tracecontext (W3C Trace Context)
	baseotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &TracerProvider{
		provider: tp,
		cleanup: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	}, nil
}

// Shutdown cleanly shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.cleanup != nil {
		return tp.cleanup(ctx)
	}
	return nil
}
