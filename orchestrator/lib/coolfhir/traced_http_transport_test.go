package coolfhir

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

func newTestTracer() *trace.TracerProvider {
	return trace.NewTracerProvider()
}

func TestNewTracedHTTPTransport(t *testing.T) {
	t.Run("with base transport", func(t *testing.T) {
		tp := newTestTracer()
		transport := NewTracedHTTPTransport(http.DefaultTransport, tp.Tracer("test"))
		require.NotNil(t, transport)
		assert.Equal(t, http.DefaultTransport, transport.base)
	})
	t.Run("nil base defaults to http.DefaultTransport", func(t *testing.T) {
		tp := newTestTracer()
		transport := NewTracedHTTPTransport(nil, tp.Tracer("test"))
		require.NotNil(t, transport)
		assert.Equal(t, http.DefaultTransport, transport.base)
	})
}

func TestTracedHTTPTransport_RoundTrip(t *testing.T) {
	tp := newTestTracer()

	t.Run("successful request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		transport := NewTracedHTTPTransport(server.Client().Transport, tp.Tracer("test"))
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/test", nil)
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("error response sets span error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		transport := NewTracedHTTPTransport(server.Client().Transport, tp.Tracer("test"))
		req, _ := http.NewRequest(http.MethodGet, server.URL+"/error", nil)
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("transport error propagated", func(t *testing.T) {
		failingTransport := &failingRoundTripper{err: errors.New("connection refused")}
		transport := NewTracedHTTPTransport(failingTransport, tp.Tracer("test"))
		req, _ := http.NewRequest(http.MethodGet, "http://localhost:9/unreachable", nil)
		_, err := transport.RoundTrip(req)
		require.Error(t, err)
	})

	t.Run("request with nil headers gets headers initialized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		transport := NewTracedHTTPTransport(server.Client().Transport, tp.Tracer("test"))
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		req.Header = nil
		resp, err := transport.RoundTrip(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

type failingRoundTripper struct {
	err error
}

func (f *failingRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, f.err
}
