package messaging

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestHTTPBroker(t *testing.T) {
	// Create a test HTTP server
	var capturedBody []byte
	var capturedContentType string
	var capturedTopic string
	var capturedHTTPMethod string
	wg := &sync.WaitGroup{}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer wg.Done()
		capturedTopic = r.URL.Path
		capturedHTTPMethod = r.Method
		capturedContentType = r.Header.Get("Content-Type")
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	// Configure the HTTPBroker to use the test server's URL
	broker := HTTPBroker{
		endpoint: testServer.URL,
	}

	message := &Message{
		Body:        []byte(`{"key":"value"}`),
		ContentType: "application/json",
	}

	t.Run("ok", func(t *testing.T) {
		wg.Add(1)
		err := broker.SendMessage(context.Background(), "test-topic", message)
		wg.Wait()
		require.NoError(t, err)
		require.JSONEq(t, `{"key":"value"}`, string(capturedBody))
		require.Equal(t, "application/json", capturedContentType)
		require.Equal(t, "/test-topic", capturedTopic)
		require.Equal(t, http.MethodPost, capturedHTTPMethod)
	})
}
