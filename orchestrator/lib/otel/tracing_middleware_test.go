package otel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestNewTracedHTTPClient(t *testing.T) {
	client := NewTracedHTTPClient("test-service")
	require.NotNil(t, client)
	assert.NotNil(t, client.Transport)
}

func TestHandlerWithTracing(t *testing.T) {
	tp := trace.NewTracerProvider()
	tracer := tp.Tracer("test")

	t.Run("successful request", func(t *testing.T) {
		handlerCalled := false
		handler := HandlerWithTracing(tracer, "test-op")(func(w http.ResponseWriter, r *http.Request) {
			handlerCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		assert.True(t, handlerCalled)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("captures non-200 status code", func(t *testing.T) {
		handler := HandlerWithTracing(tracer, "test-op")(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		req := httptest.NewRequest(http.MethodGet, "/missing", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, rw.statusCode)
	assert.Equal(t, http.StatusCreated, rec.Code)
}
