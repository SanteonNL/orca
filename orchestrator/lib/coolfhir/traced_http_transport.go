package coolfhir

import (
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	lib_otel "github.com/SanteonNL/orca/orchestrator/lib/otel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

type TracedHTTPTransport struct {
	base   http.RoundTripper
	tracer trace.Tracer
}

func NewTracedHTTPTransport(base http.RoundTripper, tracer trace.Tracer) *TracedHTTPTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &TracedHTTPTransport{
		base:   base,
		tracer: tracer,
	}
}

func (t *TracedHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := t.tracer.Start(req.Context(),
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String(lib_otel.HTTPMethod, req.Method),
			attribute.String(lib_otel.HTTPURL, req.URL.String()),
			attribute.String("http.scheme", req.URL.Scheme),
			attribute.String("http.host", req.URL.Host),
			attribute.String("http.target", req.URL.Path),
			attribute.String("user_agent.original", req.UserAgent()),
		),
	)
	defer span.End()

	// Inject trace context into HTTP headers
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Execute the request with traced context
	req = req.WithContext(ctx)
	resp, err := t.base.RoundTrip(req)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return resp, err
	}

	// Record response attributes
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.status_text", resp.Status),
	)

	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return resp, nil
}
