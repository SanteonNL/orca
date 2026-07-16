package otel

import (
	"context"
	"testing"
	"time"
)

func TestInitializeLogs_DisabledIsNoop(t *testing.T) {
	lp, err := InitializeLogs(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lp == nil {
		t.Fatal("expected a non-nil LoggerProvider")
	}
	if err := lp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestInitializeLogs_NonOTLPExporterIsNoop(t *testing.T) {
	cfg := Config{Enabled: true, Exporter: ExporterConfig{Type: "stdout"}}
	lp, err := InitializeLogs(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := lp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestInitializeLogs_OTLP(t *testing.T) {
	cfg := Config{
		Enabled:            true,
		ServiceName:        "test-logs",
		ServiceVersion:     "1.0.0",
		ResourceAttributes: map[string]string{"service.instance.id": "test-1"},
		Exporter: ExporterConfig{
			Type: "otlp",
			OTLP: OTLPConfig{
				Endpoint: "localhost:4317",
				Insecure: true,
				Timeout:  time.Second,
				Headers:  map[string]string{"x-api-key": "value"},
			},
		},
	}
	lp, err := InitializeLogs(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lp == nil || lp.provider == nil {
		t.Fatal("expected a real LoggerProvider")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := lp.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestLoggerProvider_ShutdownNilCleanup(t *testing.T) {
	lp := &LoggerProvider{}
	if err := lp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}
