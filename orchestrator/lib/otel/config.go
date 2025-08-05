package otel

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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
	// Exporter configuration
	Exporter ExporterConfig `koanf:"exporter"`
}

type ExporterConfig struct {
	// Type of exporter: "otlp", "stdout", or "none"
	Type string `koanf:"type"`
	// OTLP exporter configuration (when type is "otlp")
	OTLP OTLPConfig `koanf:"otlp"`
}

type OTLPConfig struct {
	// Endpoint for the OTLP exporter (e.g., "http://localhost:4318")
	Endpoint string `koanf:"endpoint"`
	// Headers to send with OTLP requests
	Headers map[string]string `koanf:"headers"`
	// Timeout for OTLP requests
	Timeout time.Duration `koanf:"timeout"`
	// Insecure controls whether to use HTTP instead of HTTPS
	Insecure bool `koanf:"insecure"`
}

// DefaultConfig returns a default OTEL configuration
func DefaultConfig() Config {
	return Config{
		Enabled:        false,
		ServiceName:    "orca-orchestrator",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type: "stdout",
			OTLP: OTLPConfig{
				Endpoint: "http://localhost:4318",
				Timeout:  10 * time.Second,
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
		otel.SetTracerProvider(noopProvider)
		return &TracerProvider{
			provider: noopProvider,
			cleanup:  func(context.Context) error { return nil },
		}, nil
	}

	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create exporter based on configuration
	var exporter trace.SpanExporter
	switch config.Exporter.Type {
	case "otlp":
		// Debug logging to understand what's happening
		fmt.Printf("🐛 OTLP Config - Endpoint: %s, Insecure: %t\n", config.Exporter.OTLP.Endpoint, config.Exporter.OTLP.Insecure)

		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(config.Exporter.OTLP.Endpoint),
			otlptracehttp.WithTimeout(config.Exporter.OTLP.Timeout),
		}

		// Add headers if configured
		if len(config.Exporter.OTLP.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(config.Exporter.OTLP.Headers))
		}

		// Use HTTP or HTTPS based on Insecure flag
		if config.Exporter.OTLP.Insecure {
			fmt.Printf("🐛 Adding WithInsecure() option\n")
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		// Add retry configuration to handle temporary connection issues
		opts = append(opts, otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: 1 * time.Second,
			MaxInterval:     5 * time.Second,
			MaxElapsedTime:  30 * time.Second,
		}))

		fmt.Printf("🐛 Creating OTLP exporter with endpoint: %s\n", config.Exporter.OTLP.Endpoint)

		exporter, err = otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
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
	otel.SetTracerProvider(tp)

	// Set global propagator to tracecontext (W3C Trace Context)
	otel.SetTextMapPropagator(propagation.TraceContext{})

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
