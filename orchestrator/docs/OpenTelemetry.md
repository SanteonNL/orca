# OpenTelemetry Configuration for ORCA Orchestrator

This document explains how to configure and use OpenTelemetry tracing in the ORCA orchestrator application.

## Overview

OpenTelemetry (OTEL) has been integrated centrally into the orchestrator to provide distributed tracing capabilities for both the CarePlanService and CarePlanContributor components. The implementation automatically instruments HTTP requests, including FHIR API calls.

## Configuration

OpenTelemetry is configured through environment variables with the `ORCA_OPENTELEMETRY_` prefix.

### Basic Configuration

```bash
# Enable OpenTelemetry
ORCA_OPENTELEMETRY_ENABLED=true

# Service identification
ORCA_OPENTELEMETRY_SERVICE_NAME=orca-orchestrator
ORCA_OPENTELEMETRY_SERVICE_VERSION=1.0.0

# Exporter type (stdout, otlp, or none)
ORCA_OPENTELEMETRY_EXPORTER_TYPE=stdout
```

### OTLP Exporter Configuration

For production environments, you'll typically want to use the OTLP exporter to send traces to an observability platform:

```bash
# Use OTLP exporter
ORCA_OPENTELEMETRY_EXPORTER_TYPE=otlp

# OTLP endpoint (e.g., Jaeger, OTEL Collector)
ORCA_OPENTELEMETRY_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Optional: Custom headers for authentication
ORCA_OPENTELEMETRY_EXPORTER_OTLP_HEADERS_AUTHORIZATION="Bearer your-token"

# Optional: Timeout for OTLP requests (default: 10s)
ORCA_OPENTELEMETRY_EXPORTER_OTLP_TIMEOUT=15s
```

### Stdout Exporter (Development)

For development and testing, you can use the stdout exporter to see traces in the console:

```bash
ORCA_OPENTELEMETRY_EXPORTER_TYPE=stdout
```

### Disable Tracing

To disable OpenTelemetry entirely:

```bash
ORCA_OPENTELEMETRY_ENABLED=false
```

## What Gets Traced

The implementation automatically traces:

1. **HTTP Requests**: All outbound HTTP requests from both services
2. **FHIR Operations**: Specific instrumentation for FHIR API calls with:
   - Custom span names like `fhir.get /Patient/123`
   - FHIR-specific attributes (`fhir.base_url`, `fhir.auth_type`)
3. **Distributed Context**: Trace context propagation using W3C Trace Context standard

## Span Attributes

The following attributes are automatically added to FHIR-related spans:

- `fhir.base_url`: The base URL of the FHIR server
- `fhir.auth_type`: Authentication method used (e.g., "azure-managedidentity")
- Standard HTTP attributes (method, status code, URL, etc.)

## Integration Points

### CarePlanService
All FHIR client operations in the CarePlanService are automatically instrumented when using the configured FHIR client.

### CarePlanContributor  
All FHIR client operations in the CarePlanContributor are automatically instrumented when using the configured FHIR client.

### Custom Tracing
To add custom tracing to your code, you can use the global tracer:

```go
import "go.opentelemetry.io/otel"

func MyFunction(ctx context.Context) error {
    tracer := otel.Tracer("my-component")
    ctx, span := tracer.Start(ctx, "my-operation")
    defer span.End()
    
    // Your code here
    
    return nil
}
```

## Examples

### Local Development with Jaeger

```bash
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 4317:4317 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest

# Configure ORCA
ORCA_OPENTELEMETRY_ENABLED=true
ORCA_OPENTELEMETRY_SERVICE_NAME=orca-orchestrator
ORCA_OPENTELEMETRY_EXPORTER_TYPE=otlp
ORCA_OPENTELEMETRY_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

### Production with OTEL Collector

```bash
ORCA_OPENTELEMETRY_ENABLED=true
ORCA_OPENTELEMETRY_SERVICE_NAME=orca-orchestrator
ORCA_OPENTELEMETRY_SERVICE_VERSION=2.1.0
ORCA_OPENTELEMETRY_EXPORTER_TYPE=otlp
ORCA_OPENTELEMETRY_EXPORTER_OTLP_ENDPOINT=https://your-otel-collector:4318
ORCA_OPENTELEMETRY_EXPORTER_OTLP_HEADERS_AUTHORIZATION="Bearer your-api-key"
```

## Troubleshooting

1. **No traces appearing**: Check that `ORCA_OPENTELEMETRY_ENABLED=true` is set
2. **OTLP connection issues**: Verify the endpoint URL and network connectivity
3. **Authentication errors**: Check that headers are properly configured
4. **Performance impact**: Tracing has minimal overhead, but you can disable it in production if needed

## Architecture Notes

- OpenTelemetry is initialized once at application startup in `cmd/server.go`
- The tracer provider is configured globally and shared across all components
- Graceful shutdown ensures all traces are flushed before the application exits
- The implementation follows OpenTelemetry best practices for Go applications
