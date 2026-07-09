package logging

import (
	"context"
	"errors"
	"log/slog"
)

// FanoutHandler dispatches a single slog.Record to multiple downstream handlers,
// so we can keep JSON stdout output while also emitting to the OpenTelemetry log bridge.
type FanoutHandler struct {
	Handlers []slog.Handler
}

func (h FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, hh := range h.Handlers {
		if hh.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, hh := range h.Handlers {
		if !hh.Enabled(ctx, r.Level) {
			continue
		}
		if err := hh.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.Handlers))
	for i, hh := range h.Handlers {
		handlers[i] = hh.WithAttrs(attrs)
	}
	return FanoutHandler{Handlers: handlers}
}

func (h FanoutHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.Handlers))
	for i, hh := range h.Handlers {
		handlers[i] = hh.WithGroup(name)
	}
	return FanoutHandler{Handlers: handlers}
}
