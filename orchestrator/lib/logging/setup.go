package logging

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// NewApplicationHandler builds the root slog handler for the application.
//
// Functional logs fan out to:
//   - stdout (JSON), for local dev and Azure Container Apps console capture, and
//   - the OTEL logs bridge (only when otelEnabled), gated to Info and above so that Debug
//     diagnostics — which may contain PII — stay on stdout only and never reach App Insights.
//
// Records are wrapped in ContextHandler so trace/span IDs are attached for correlation.
func NewApplicationHandler(level slog.Leveler, otelServiceName string, otelEnabled bool) slog.Handler {
	handlers := []slog.Handler{
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: true,
		}),
	}
	if otelEnabled {
		handlers = append(handlers, NewLevelHandler(slog.LevelInfo, otelslog.NewHandler(otelServiceName)))
	}
	return &ContextHandler{Handler: NewFanoutHandler(handlers...)}
}
