package pipeline

import (
	"context"
	"github.com/stretchr/testify/require"
	baseotel "go.opentelemetry.io/otel"
	"net/http/httptest"
	"testing"
)

func TestInstance_Do(t *testing.T) {
	t.Run("marshals response", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()
		resource := map[string]interface{}{
			"foo": "bar",
		}

		New().DoAndWrite(context.Background(), baseotel.Tracer("test"), httpResponse, resource, 200)

		require.JSONEq(t, `{"foo":"bar"}`, httpResponse.Body.String())
		require.Equal(t, "application/fhir+json", httpResponse.Header().Get("Content-Type"))
		require.Equal(t, "18", httpResponse.Header().Get("Content-Length"))
		require.Equal(t, 200, httpResponse.Code)
	})
	t.Run("doesn't re-marshal byte slice", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()

		New().DoAndWrite(context.Background(), baseotel.Tracer("test"), httpResponse, []byte(`{"foo":"bar"}`), 200)

		require.Equal(t, `{"foo":"bar"}`, httpResponse.Body.String())
		require.Equal(t, "application/fhir+json", httpResponse.Header().Get("Content-Type"))
		require.Equal(t, "13", httpResponse.Header().Get("Content-Length"))
		require.Equal(t, 200, httpResponse.Code)
	})
	t.Run("runs response transformers", func(t *testing.T) {
		httpResponse := httptest.NewRecorder()
		resource := map[string]interface{}{
			"foo": "bar",
		}
		var fn stubHttpResponseTransformer = func(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string) {
			*responseStatus = 201
			*responseBody = []byte(`{"foo":"bazzzz"}`)
			responseHeaders["Content-Type"] = []string{"application/something+json"}
		}

		New().AppendResponseTransformer(fn).DoAndWrite(context.Background(), baseotel.Tracer("test"), httpResponse, resource, 200)

		require.JSONEq(t, `{"foo":"bazzzz"}`, httpResponse.Body.String())
		require.Equal(t, "application/something+json", httpResponse.Header().Get("Content-Type"))
		require.Equal(t, "16", httpResponse.Header().Get("Content-Length"))
		require.Equal(t, 201, httpResponse.Code)
	})
}

type stubHttpResponseTransformer func(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string)

func (s stubHttpResponseTransformer) Transform(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string) {
	s(responseStatus, responseBody, responseHeaders)
}
