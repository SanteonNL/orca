package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	baseotel "go.opentelemetry.io/otel"
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

func TestInstance_Do_NilResource(t *testing.T) {
	httpResponse := &http.Response{Header: http.Header{}, StatusCode: 200, Body: nil}
	err := New().Do(context.Background(), baseotel.Tracer("test"), httpResponse, nil)
	require.NoError(t, err)
}

func TestInstance_Do_IOReader(t *testing.T) {
	httpResponse := httptest.NewRecorder()
	body := strings.NewReader(`{"foo":"bar"}`)
	New().DoAndWrite(context.Background(), baseotel.Tracer("test"), httpResponse, body, 200)
	require.Equal(t, `{"foo":"bar"}`, httpResponse.Body.String())
}

func TestInstance_PrependResponseTransformer(t *testing.T) {
	httpResponse := httptest.NewRecorder()
	var order []string
	first := stubHttpResponseTransformer(func(_ *int, _ *[]byte, _ map[string][]string) { order = append(order, "first") })
	second := stubHttpResponseTransformer(func(_ *int, _ *[]byte, _ map[string][]string) { order = append(order, "second") })

	New().AppendResponseTransformer(second).PrependResponseTransformer(first).
		DoAndWrite(context.Background(), baseotel.Tracer("test"), httpResponse, []byte(`{}`), 200)

	require.Equal(t, []string{"first", "second"}, order)
}

type stubHttpResponseTransformer func(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string)

func (s stubHttpResponseTransformer) Transform(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string) {
	s(responseStatus, responseBody, responseHeaders)
}
