package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestFanoutHandler_LevelGating verifies that each child handler filters by its own level:
// a Debug-level stdout handler receives Debug records, while an Info-gated handler (the OTEL
// bridge stand-in) does not. This is the mechanism that keeps Debug PII off Application Insights.
func TestFanoutHandler_LevelGating(t *testing.T) {
	var debugSink, infoSink bytes.Buffer

	debugHandler := slog.NewJSONHandler(&debugSink, &slog.HandlerOptions{Level: slog.LevelDebug})
	infoHandler := NewLevelHandler(slog.LevelInfo, slog.NewJSONHandler(&infoSink, &slog.HandlerOptions{Level: slog.LevelDebug}))

	logger := slog.New(NewFanoutHandler(debugHandler, infoHandler))

	logger.Debug("debug-only-secret")
	logger.Info("info-visible")

	debugOut := debugSink.String()
	infoOut := infoSink.String()

	if !strings.Contains(debugOut, "debug-only-secret") {
		t.Fatalf("debug handler should have received the debug record, got: %q", debugOut)
	}
	if !strings.Contains(debugOut, "info-visible") {
		t.Fatalf("debug handler should have received the info record, got: %q", debugOut)
	}
	if strings.Contains(infoOut, "debug-only-secret") {
		t.Fatalf("info-gated handler must NOT receive debug records (PII gate leak), got: %q", infoOut)
	}
	if !strings.Contains(infoOut, "info-visible") {
		t.Fatalf("info-gated handler should have received the info record, got: %q", infoOut)
	}
}

// TestLevelHandler_Enabled verifies the level threshold.
func TestLevelHandler_Enabled(t *testing.T) {
	h := NewLevelHandler(slog.LevelInfo, slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelDebug) {
		t.Fatal("Debug should be disabled below the Info threshold")
	}
	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Fatal("Info should be enabled at the Info threshold")
	}
	if !h.Enabled(ctx, slog.LevelError) {
		t.Fatal("Error should be enabled above the Info threshold")
	}
}
