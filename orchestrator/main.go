package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/SanteonNL/orca/orchestrator/cmd"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

func main() {
	config, err := cmd.LoadConfig()
	if err != nil {
		// Configuration failures are technical/pre-init: log to stderr only, not OTEL.
		slog.New(slog.NewJSONHandler(os.Stderr, nil)).
			Error("Failed to load configuration", slog.String(logging.FieldError, err.Error()))
		os.Exit(1)
	}
	// Set up the global slog logger (stdout + OTEL logs bridge; see NewApplicationHandler).
	otelEnabled := config.OpenTelemetry.Enabled && config.OpenTelemetry.Exporter.Type == "otlp"
	slog.SetDefault(slog.New(logging.NewApplicationHandler(config.LogLevel, config.OpenTelemetry.ServiceName, otelEnabled)))
	slog.Info(fmt.Sprintf("Public interface listens on %s", config.Public.Address))
	slog.Info(fmt.Sprintf("Using Nuts API on %s", config.Nuts.API.URL))
	if err := cmd.Start(context.Background(), *config); err != nil {
		slog.Error("Failed to start server", slog.String(logging.FieldError, err.Error()))
		os.Exit(1)
	}
	slog.Info("Goodbye!")
}
