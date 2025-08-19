package otel

import (
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	"net/http"
)

func HandlerWithTracing(tracer trace.Tracer, operationName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(
			r.Context(),
			operationName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPURLKey.String(r.URL.String()),
				semconv.HostNameKey.String(r.Host),
				attribute.String("tenant.id", getTenantFromRequest(r)),
			),
		)
		defer span.End()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		handler(wrapped, r.WithContext(ctx))

		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(wrapped.statusCode))
		if wrapped.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}

func TracingMiddleware(mux *http.ServeMux, tracerName string) *http.ServeMux {
	tracedMux := http.NewServeMux()

	// Wrap the original mux with tracing middleware
	tracedMux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracer := otel.Tracer(tracerName)
		ctx, span := tracer.Start(
			r.Context(),
			fmt.Sprintf("%s %s", r.Method, r.URL.Path),
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPMethodKey.String(r.Method),
				semconv.HTTPURLKey.String(r.URL.String()),
				semconv.HostNameKey.String(r.Host),
				attribute.String("tenant.id", getTenantFromRequest(r)),
			),
		)
		defer span.End()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		mux.ServeHTTP(wrapped, r.WithContext(ctx))

		span.SetAttributes(semconv.HTTPStatusCodeKey.Int(wrapped.statusCode))
		if wrapped.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}))

	return tracedMux
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func getTenantFromRequest(r *http.Request) string {
	if tenant := r.PathValue("tenant"); tenant != "" {
		return tenant
	}
	return "unknown"
}
