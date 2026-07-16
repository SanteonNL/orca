package otel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	otellogglobal "go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// LoggerProvider holds the global OTEL logger provider and its cleanup function.
type LoggerProvider struct {
	provider *sdklog.LoggerProvider
	cleanup  func(context.Context) error
}

// InitializeLogs sets up the OpenTelemetry logs pipeline so that (functional) slog records
// bridged via otelslog are exported to the OTLP endpoint and, in Azure Container Apps,
// forwarded to Application Insights.
//
// It only wires a real exporter when OpenTelemetry is enabled and the OTLP exporter is
// selected; otherwise it installs a no-op so the otelslog handler simply drops records
// (they still reach stdout via the fan-out handler). It reuses the same OTLP endpoint and
// resource attributes as the trace pipeline (see Initialize) for consistent correlation.
func InitializeLogs(ctx context.Context, config Config) (*LoggerProvider, error) {
	noop := &LoggerProvider{cleanup: func(context.Context) error { return nil }}
	if !config.Enabled || config.Exporter.Type != "otlp" {
		return noop, nil
	}

	// Build the same resource as the trace pipeline (service name/version + extra attrs).
	resourceOpts := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		),
	}
	if len(config.ResourceAttributes) > 0 {
		var attrs []attribute.KeyValue
		for key, value := range config.ResourceAttributes {
			attrs = append(attrs, attribute.String(key, value))
		}
		resourceOpts = append(resourceOpts, resource.WithAttributes(attrs...))
	}
	res, err := resource.New(ctx, resourceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource for logs: %w", err)
	}

	opts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(config.Exporter.OTLP.Endpoint),
		otlploggrpc.WithTimeout(config.Exporter.OTLP.Timeout),
	}
	if config.Exporter.OTLP.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	if len(config.Exporter.OTLP.Headers) > 0 {
		opts = append(opts, otlploggrpc.WithHeaders(config.Exporter.OTLP.Headers))
	}

	exporter, err := otlploggrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	// Set the global logger provider so otelslog handlers (constructed at startup) delegate to it.
	otellogglobal.SetLoggerProvider(lp)

	return &LoggerProvider{
		provider: lp,
		cleanup: func(ctx context.Context) error {
			return lp.Shutdown(ctx)
		},
	}, nil
}

// Shutdown flushes and stops the logger provider.
func (lp *LoggerProvider) Shutdown(ctx context.Context) error {
	if lp.cleanup != nil {
		return lp.cleanup(ctx)
	}
	return nil
}
