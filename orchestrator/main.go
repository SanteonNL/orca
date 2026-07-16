package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/SanteonNL/orca/orchestrator/cmd"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"go.opentelemetry.io/contrib/bridges/otelslog"
)

func main() {
	config, err := cmd.LoadConfig()
	if err != nil {
		// Configuration failures are technical/pre-init: log to stderr only, not OTEL.
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("Failed to load configuration", slog.String(logging.FieldError, err.Error()))
		os.Exit(1)
	}
	// Set up the global slog logger. Functional logs fan out to:
	//  - stdout (JSON), for local dev and Azure Container Apps console capture, and
	//  - the OTEL logs bridge (Info+ only), which is forwarded to Application Insights.
	// The Info gate keeps Debug diagnostics (which may contain PII) on stdout only.
	// The ContextHandler adds trace/span IDs so log records correlate with traces.
	handlers := []slog.Handler{
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     config.LogLevel,
			AddSource: true,
		}),
	}
	if config.OpenTelemetry.Enabled && config.OpenTelemetry.Exporter.Type == "otlp" {
		handlers = append(handlers, logging.NewLevelHandler(
			slog.LevelInfo,
			otelslog.NewHandler(config.OpenTelemetry.ServiceName),
		))
	}
	h := &logging.ContextHandler{Handler: logging.NewFanoutHandler(handlers...)}
	slog.SetDefault(slog.New(h))
	slog.Info(fmt.Sprintf("Public interface listens on %s", config.Public.Address))
	slog.Info(fmt.Sprintf("Using Nuts API on %s", config.Nuts.API.URL))
	if err := cmd.Start(context.Background(), *config); err != nil {
		slog.Error("Failed to start server", slog.String(logging.FieldError, err.Error()))
		os.Exit(1)
	}
	slog.Info("Goodbye!")
}
