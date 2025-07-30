package main

import (
	"fmt"
	"strings"

	testcontainers "github.com/testcontainers/testcontainers-go"
)

// FilteredLogConsumer wraps testcontainers log consumer to filter and categorize logs
type FilteredLogConsumer struct {
	containerName string
	logFilter     *LogFilter
}

func NewFilteredLogConsumer(containerName string) *FilteredLogConsumer {
	return &FilteredLogConsumer{
		containerName: containerName,
		logFilter:     NewEnhancedLogFilter(),
	}
}

func (f *FilteredLogConsumer) Accept(l testcontainers.Log) {
	logLine := string(l.Content)
	logLine = strings.TrimSpace(logLine)

	if logLine == "" {
		return
	}

	// Categorize the log
	if f.logFilter.IsOTELLog(logLine) {
		fmt.Printf("🔍 [OTEL-%s] %s\n", f.containerName, logLine)
	} else {
		fmt.Printf("📋 [APP-%s] %s\n", f.containerName, logLine)
	}
}

// Enhanced log filter with more comprehensive patterns
func NewEnhancedLogFilter() *LogFilter {
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
