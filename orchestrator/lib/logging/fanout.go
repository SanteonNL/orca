package logging

import (
	"context"
	"errors"
	"log/slog"
)

// FanoutHandler dispatches each log record to every child handler that is enabled for the
// record's level. Each child applies its own level filtering via its Enabled method, so a
// stdout handler can log at Debug while an OTEL bridge handler only forwards Info and above.
type FanoutHandler struct {
	handlers []slog.Handler
}

// NewFanoutHandler creates a FanoutHandler over the given child handlers.
func NewFanoutHandler(handlers ...slog.Handler) *FanoutHandler {
	return &FanoutHandler{handlers: handlers}
}

func (h *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, c := range h.handlers {
		if c.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *FanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, c := range h.handlers {
		if c.Enabled(ctx, r.Level) {
			// Clone so each handler can safely mutate its own copy of the record.
			if err := c.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (h *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, c := range h.handlers {
		next[i] = c.WithAttrs(attrs)
	}
	return &FanoutHandler{handlers: next}
}

func (h *FanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.handlers))
	for i, c := range h.handlers {
		next[i] = c.WithGroup(name)
	}
	return &FanoutHandler{handlers: next}
}

// LevelHandler wraps a handler, only enabling records at or above a minimum level. It is
// used to gate the OTEL logs bridge to Info+ so that Debug diagnostics (which may contain
// PII) stay on stdout only and are never exported to Application Insights.
type LevelHandler struct {
	min     slog.Leveler
	handler slog.Handler
}

// NewLevelHandler wraps handler so it only handles records at or above min.
func NewLevelHandler(min slog.Leveler, handler slog.Handler) *LevelHandler {
	return &LevelHandler{min: min, handler: handler}
}

func (h *LevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.min.Level() && h.handler.Enabled(ctx, level)
}

func (h *LevelHandler) Handle(ctx context.Context, r slog.Record) error {
	return h.handler.Handle(ctx, r)
}

func (h *LevelHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &LevelHandler{min: h.min, handler: h.handler.WithAttrs(attrs)}
}

func (h *LevelHandler) WithGroup(name string) slog.Handler {
	return &LevelHandler{min: h.min, handler: h.handler.WithGroup(name)}
}
