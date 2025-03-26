package sse

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeResponseWriter is a custom ResponseWriter that supports http.Flusher.
type fakeResponseWriter struct {
	header  http.Header
	mu      sync.Mutex
	buf     bytes.Buffer
	flushed chan struct{}
}

func newFakeResponseWriter() *fakeResponseWriter {
	return &fakeResponseWriter{
		header:  make(http.Header),
		flushed: make(chan struct{}, 100),
	}
}

func (w *fakeResponseWriter) Header() http.Header {
	return w.header
}

func (w *fakeResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(b)
}

func (w *fakeResponseWriter) WriteHeader(statusCode int) {
	// For testing we ignore the status code.
}

func (w *fakeResponseWriter) Flush() {
	// Signal that a flush has occurred.
	w.flushed <- struct{}{}
}

// readOutput returns the current written output.
func (w *fakeResponseWriter) readOutput() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// TestServeHTTP_Publishes tests that messages published on a topic
// are received by a connected SSE client.
func TestServeHTTP_Publishes(t *testing.T) {
	s := New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create a fake request with our cancellable context.
	req := httptest.NewRequest("GET", "/subscribe/test", nil)
	req = req.WithContext(ctx)
	// Create our fake response writer.
	frw := newFakeResponseWriter()

	// Run ServeHTTP in a goroutine.
	done := make(chan struct{})
	go func() {
		s.ServeHTTP("test", frw, req)
		close(done)
	}()

	// Give the ServeHTTP loop time to start and register the client.
	time.Sleep(10 * time.Millisecond)

	// Check that the initial ping is sent
	output := frw.readOutput()
	expected := "ping\n\n"
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got %q", expected, output)
	}

	// Publish a message.
	msg := "hello world"
	s.Publish("test", msg)

	// Wait for a flush to occur (i.e. message has been written).
	select {
	case <-frw.flushed:
		// message flushed.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for flush")
	}

	// Give the ServeHTTP loop time to send the async message.
	time.Sleep(10 * time.Millisecond)

	// Check that the output contains our published message.
	output = frw.readOutput()
	expected = "data: " + msg + "\n\n"
	if !strings.Contains(output, expected) {
		t.Errorf("expected output to contain %q, got %q", expected, output)
	}

	// Cancel the request context so that ServeHTTP loop ends.
	cancel()

	// Wait for ServeHTTP to return.
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ServeHTTP did not exit after context cancellation")
	}
}

// TestPublish_ChannelFull tests the Publish method when client channel buffer is full.
// The message is dropped in this case.
func TestPublish_ChannelFull(t *testing.T) {
	s := New()

	// Create a dummy client channel with a small buffer.
	ch := make(chan string, 1)
	// Register the client under topic "full".
	s.registerClient("full", ch)

	// Fill the channel.
	ch <- "first"
	// At this point, the channel is full (buffer size is 1).
	// Publish a message which should be dropped.
	s.Publish("full", "dropped")

	// Read from channel: should only receive the first message.
	select {
	case msg := <-ch:
		if msg != "first" {
			t.Errorf("expected message 'first', got %q", msg)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected to receive a message")
	}

	// There should be no second message.
	select {
	case msg := <-ch:
		t.Errorf("unexpected message received: %q", msg)
	case <-time.After(100 * time.Millisecond):
		// expected no message.
	}

	// Clean up.
	s.unregisterClient("full", ch)
}

// TestRegisterAndUnregisterClient tests that registerClient and unregisterClient add and remove clients appropriately.
func TestRegisterAndUnregisterClient(t *testing.T) {
	s := New()
	topic := "testTopic"
	ch := make(chan string, 10)

	// Initially, topic should not exist.
	s.mu.RLock()
	if _, exists := s.clients[topic]; exists {
		t.Errorf("expected topic %q not to exist", topic)
	}
	s.mu.RUnlock()

	// Register client.
	s.registerClient(topic, ch)

	s.mu.RLock()
	clients, exists := s.clients[topic]
	s.mu.RUnlock()
	if !exists {
		t.Fatalf("expected topic %q to exist after registration", topic)
	}
	if _, exists = clients[ch]; !exists {
		t.Errorf("expected channel to be registered under topic %q", topic)
	}

	// Unregister client.
	s.unregisterClient(topic, ch)

	s.mu.RLock()
	clients, exists = s.clients[topic]
	s.mu.RUnlock()
	// Channel should be removed.
	if exists {
		if _, exists := clients[ch]; exists {
			t.Errorf("expected channel to be removed from topic %q", topic)
		}
	}
	// Also, channel should be closed.
	_, open := <-ch
	if open {
		t.Errorf("expected channel to be closed upon unregistration")
	}
}

// TestUnregisterClient_RemovesTopicWhenEmpty verifies that when the last client is unregistered,
// the topic is removed from the clients map.
func TestUnregisterClient_RemovesTopicWhenEmpty(t *testing.T) {
	s := New()
	topic := "removeTopic"
	ch := make(chan string, 10)

	s.registerClient(topic, ch)
	s.unregisterClient(topic, ch)

	s.mu.RLock()
	_, exists := s.clients[topic]
	s.mu.RUnlock()
	if exists {
		t.Errorf("expected topic %q to be removed after last client unregistration", topic)
	}
}

// TestUnregisterClient_MultipleClients tests that unregistering one of multiple clients does not remove the topic,
// and that the topic is removed only after the last client is unregistered.
func TestUnregisterClient_MultipleClients(t *testing.T) {
	s := New()
	topic := "multiTopic"
	ch1 := make(chan string, 10)
	ch2 := make(chan string, 10)

	s.registerClient(topic, ch1)
	s.registerClient(topic, ch2)

	// Unregister one client; the topic should still exist.
	s.unregisterClient(topic, ch1)

	s.mu.RLock()
	clients, exists := s.clients[topic]
	if !exists {
		t.Fatalf("expected topic %q to exist", topic)
	}
	if len(clients) != 1 {
		t.Errorf("expected 1 client remaining in topic %q, got %d", topic, len(clients))
	}
	s.mu.RUnlock()

	// Unregister the remaining client; the topic should be removed.
	s.unregisterClient(topic, ch2)

	s.mu.RLock()
	_, exists = s.clients[topic]
	s.mu.RUnlock()
	if exists {
		t.Errorf("expected topic %q to be removed after unregistering all clients", topic)
	}
}
