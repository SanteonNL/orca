package coolfhir

import (
	"errors"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	baseotel "go.opentelemetry.io/otel"
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
			attribute.String(otel.HTTPMethod, req.Method),
			attribute.String(otel.HTTPURL, req.URL.String()),
			attribute.String("http.scheme", req.URL.Scheme),
			attribute.String("http.host", req.URL.Host),
			attribute.String("http.target", req.URL.Path),
			attribute.String("user_agent.original", req.UserAgent()),
		),
	)
	defer span.End()

	// Clone the request to avoid modifying the original
	reqClone := req.Clone(ctx)

	// Ensure headers map exists
	if reqClone.Header == nil {
		reqClone.Header = make(http.Header)
	}

	// Inject trace context into HTTP headers
	baseotel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(reqClone.Header))

	// Execute the request with traced context
	resp, err := t.base.RoundTrip(reqClone)

	if err != nil {
		return resp, otel.Error(span, err)
	}

	// Record response attributes
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.status_text", resp.Status),
	)

	if resp.StatusCode >= 400 {
		otel.Error(span, errors.New(http.StatusText(resp.StatusCode)))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return resp, nil
}
