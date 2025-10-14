package otel

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
)

func NewTracedHTTPClient(spanNameBase string) *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(
			http.DefaultTransport,
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				return fmt.Sprintf("%s.%s %s %s", spanNameBase, operation, strings.ToLower(r.Method), r.URL.Path)
			}),
			otelhttp.WithSpanOptions(
				trace.WithSpanKind(trace.SpanKindClient),
			),
		),
	}
}

func HandlerWithTracing(tracer trace.Tracer, operationName string) func(http.HandlerFunc) http.HandlerFunc {
	return func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming HTTP headers
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			ctx, span := tracer.Start(
				ctx,
				operationName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethodKey.String(r.Method),
					semconv.HTTPURLKey.String(r.URL.String()),
					semconv.HostNameKey.String(r.Host),
				),
			)
			defer span.End()

			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			handler(wrapped, r.WithContext(ctx))

			span.SetAttributes(semconv.HTTPStatusCodeKey.Int(wrapped.statusCode))
			span.SetStatus(codes.Ok, "")
		}
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
