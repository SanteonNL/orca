package otel

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"strings"
)

// Error records an error on the span, sets the span status to error, and returns the original error.
// If message is provided, it will be used as the span status description, otherwise err.Error() is used.
func Error(span trace.Span, err error, message ...string) error {
	if err == nil {
		return nil
	}

	span.RecordError(err)

	var statusDesc string
	if len(message) > 0 && message[0] != "" {
		statusDesc = strings.Join(message, ",")
	} else {
		statusDesc = err.Error()
	}

	span.SetStatus(codes.Error, statusDesc)
	return err
}
