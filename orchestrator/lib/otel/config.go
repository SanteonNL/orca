package otel

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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
	// Logging configuration
	Logging LoggingConfig `koanf:"logging"`
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

type LoggingConfig struct {
	// Enabled controls whether OpenTelemetry logging is enabled
	Enabled bool `koanf:"enabled"`
	// Type of log exporter: "otlp", "stdout", or "none"
	ExporterType string `koanf:"exporter_type"`
	// Protocol for log OTLP exporter: "grpc" or "http"
	Protocol string `koanf:"protocol"`
	// OTLP configuration (when exporter_type is "otlp")
	OTLP OTLPLogConfig `koanf:"otlp"`
}

type OTLPLogConfig struct {
	// Endpoint for the OTLP log exporter
	Endpoint string `koanf:"endpoint"`
	// Headers to send with OTLP log requests
	Headers map[string]string `koanf:"headers"`
	// Timeout for OTLP log requests
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
		Logging: LoggingConfig{
			Enabled:      true,
			ExporterType: "otlp",
			Protocol:     protocol,
			OTLP: OTLPLogConfig{
				Endpoint: func() string {
					if loggingEndpoint != "" {
						return loggingEndpoint
					}
					return endpoint
				}(),
				Insecure: insecure,
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

// OtelProvider holds the OpenTelemetry tracer and log providers with cleanup function
type OtelProvider struct {
	tracerProvider *trace.TracerProvider
	logProvider    *log.LoggerProvider
	cleanup        func(context.Context) error
}

// Initialize sets up OpenTelemetry based on the configuration
func Initialize(ctx context.Context, config Config) (*OtelProvider, error) {
	if !config.Enabled {
		// Set up a no-op tracer provider
		noopTracerProvider := trace.NewTracerProvider()
		baseotel.SetTracerProvider(noopTracerProvider)

		// Set up a no-op log provider
		noopLogProvider := log.NewLoggerProvider()
		global.SetLoggerProvider(noopLogProvider)

		return &OtelProvider{
			tracerProvider: noopTracerProvider,
			logProvider:    noopLogProvider,
			cleanup:        func(context.Context) error { return nil },
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

	// Initialize trace provider
	traceProvider, err := initializeTraceProvider(ctx, config, res)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize trace provider: %w", err)
	}

	// Initialize log provider
	logProvider, err := initializeLogProvider(ctx, config, res)
	if err != nil {
		// Clean up trace provider if log provider fails
		if shutdownErr := traceProvider.Shutdown(ctx); shutdownErr != nil {
			return nil, fmt.Errorf("failed to initialize log provider: %w, and failed to cleanup trace provider: %w", err, shutdownErr)
		}
		return nil, fmt.Errorf("failed to initialize log provider: %w", err)
	}

	// Set global providers
	baseotel.SetTracerProvider(traceProvider)
	global.SetLoggerProvider(logProvider)

	// Set global propagator to tracecontext (W3C Trace Context)
	baseotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &OtelProvider{
		tracerProvider: traceProvider,
		logProvider:    logProvider,
		cleanup: func(ctx context.Context) error {
			var errs []error
			if err := traceProvider.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("trace provider shutdown: %w", err))
			}
			if err := logProvider.Shutdown(ctx); err != nil {
				errs = append(errs, fmt.Errorf("log provider shutdown: %w", err))
			}
			if len(errs) > 0 {
				return fmt.Errorf("shutdown errors: %v", errs)
			}
			return nil
		},
	}, nil
}

func initializeTraceProvider(ctx context.Context, config Config, res *resource.Resource) (*trace.TracerProvider, error) {
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

			var err error
			exporter, err = otlptracegrpc.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP gRPC trace exporter: %w", err)
			}

		default:
			return nil, fmt.Errorf("unsupported OTLP protocol: %s (supported: grpc)", config.Exporter.Protocol)
		}

	case "stdout":
		var err error
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}
	case "none":
		// No exporter, traces will be collected but not exported
		exporter = nil
	default:
		return nil, fmt.Errorf("unsupported trace exporter type: %s", config.Exporter.Type)
	}

	// Create tracer provider
	var opts []trace.TracerProviderOption
	opts = append(opts, trace.WithResource(res))

	if exporter != nil {
		opts = append(opts, trace.WithBatcher(exporter))
	}

	tp := trace.NewTracerProvider(opts...)
	return tp, nil
}

func initializeLogProvider(ctx context.Context, config Config, res *resource.Resource) (*log.LoggerProvider, error) {
	if !config.Logging.Enabled {
		// Return no-op log provider if logging is disabled
		return log.NewLoggerProvider(), nil
	}

	// Create log exporter based on configuration
	var exporter log.Exporter
	switch config.Logging.ExporterType {
	case "otlp":
		// Choose exporter based on protocol
		switch config.Logging.Protocol {
		case "grpc":
			opts := []otlploggrpc.Option{
				otlploggrpc.WithEndpoint(config.Logging.OTLP.Endpoint),
				otlploggrpc.WithTimeout(config.Logging.OTLP.Timeout),
			}

			// Add headers if configured
			if len(config.Logging.OTLP.Headers) > 0 {
				opts = append(opts, otlploggrpc.WithHeaders(config.Logging.OTLP.Headers))
			}

			// Use insecure gRPC connection if configured
			if config.Logging.OTLP.Insecure {
				opts = append(opts, otlploggrpc.WithInsecure())
			}

			var err error
			exporter, err = otlploggrpc.New(ctx, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to create OTLP gRPC log exporter: %w", err)
			}

		default:
			return nil, fmt.Errorf("unsupported OTLP log protocol: %s (supported: grpc)", config.Logging.Protocol)
		}

	case "stdout":
		var err error
		exporter, err = stdoutlog.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout log exporter: %w", err)
		}
	case "none":
		// No exporter, logs will be collected but not exported
		exporter = nil
	default:
		return nil, fmt.Errorf("unsupported log exporter type: %s", config.Logging.ExporterType)
	}

	// Create log provider
	var opts []log.LoggerProviderOption
	opts = append(opts, log.WithResource(res))

	if exporter != nil {
		opts = append(opts, log.WithProcessor(log.NewBatchProcessor(exporter)))
	}

	lp := log.NewLoggerProvider(opts...)
	return lp, nil
}

// Shutdown cleanly shuts down the OpenTelemetry providers
func (op *OtelProvider) Shutdown(ctx context.Context) error {
	if op.cleanup != nil {
		return op.cleanup(ctx)
	}
	return nil
}
