package messaging

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPBroker(t *testing.T) {
	// Create a test HTTP server
	var capturedBody []byte
	var capturedContentType string
	var capturedTopic string
	var capturedHTTPMethod string
	testServer := httptest.NewServer(http.HandlerFunc(func(httpResponse http.ResponseWriter, httpRequest *http.Request) {
		capturedTopic = httpRequest.URL.Path
		capturedHTTPMethod = httpRequest.Method
		capturedContentType = httpRequest.Header.Get("Content-Type")
		var err error
		capturedBody, err = io.ReadAll(httpRequest.Body)
		if err != nil {
			httpResponse.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.Contains(httpRequest.URL.Path, "500") {
			httpResponse.WriteHeader(http.StatusInternalServerError)
			return
		}
		httpResponse.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Configure the HTTPBroker to use the test server's URL
	broker := HTTPBroker{
		endpoint:    testServer.URL,
		topicFilter: []string{"test-topic", "test-topic/500"},
	}

	message := &Message{
		Body:        []byte(`{"key":"value"}`),
		ContentType: "application/json",
	}

	t.Run("ok", func(t *testing.T) {
		err := broker.SendMessage(context.Background(), Topic{Name: "test-topic"}, message)
		require.NoError(t, err)
		require.JSONEq(t, `{"key":"value"}`, string(capturedBody))
		require.Equal(t, "application/json", capturedContentType)
		require.Equal(t, "/test-topic", capturedTopic)
		require.Equal(t, http.MethodPost, capturedHTTPMethod)
	})
	t.Run("non-200 OK response", func(t *testing.T) {
		err := broker.SendMessage(context.Background(), Topic{Name: "test-topic/500"}, message)
		require.Error(t, err)
	})
	t.Run("topic filtered out (not configured)", func(t *testing.T) {
		capturedBody = nil
		err := broker.SendMessage(context.Background(), Topic{Name: "other-topic"}, message)
		require.NoError(t, err)
		require.Empty(t, capturedBody)
	})
	t.Run("no filter configured", func(t *testing.T) {
		capturedBody = nil
		broker := HTTPBroker{
			endpoint: testServer.URL,
		}
		err := broker.SendMessage(context.Background(), Topic{Name: "test-topic"}, message)
		require.NoError(t, err)
		require.NoError(t, err)
		require.NotEmpty(t, capturedBody)
	})
}
