package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewApplicationHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("otel disabled", func(t *testing.T) {
		h := NewApplicationHandler(slog.LevelInfo, "svc", false)
		if h == nil {
			t.Fatal("expected a non-nil handler")
		}
		if !h.Enabled(ctx, slog.LevelInfo) {
			t.Fatal("Info should be enabled")
		}
		if h.Enabled(ctx, slog.LevelDebug) {
			t.Fatal("Debug should be disabled when the stdout level is Info")
		}
	})

	t.Run("otel enabled", func(t *testing.T) {
		h := NewApplicationHandler(slog.LevelDebug, "svc", true)
		if h == nil {
			t.Fatal("expected a non-nil handler")
		}
		// Debug is enabled because the stdout child logs at Debug (the OTEL child stays Info+).
		if !h.Enabled(ctx, slog.LevelDebug) {
			t.Fatal("Debug should be enabled via the stdout child")
		}
	})
}
