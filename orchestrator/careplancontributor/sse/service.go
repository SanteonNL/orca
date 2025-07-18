package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
)

// This service maps a topic to a set of clients that are interested in receiving
// messages for that topic. Clients can subscribe to a topic by connecting to the
// server-sent events (SSE) endpoint for that topic. The server can then publish
// messages to all clients subscribed to a topic.
type Service struct {
	mu        sync.RWMutex
	clients   map[string]map[chan string]struct{} // topic -> clients
	ServeHTTP func(topic string, writer http.ResponseWriter, request *http.Request)
}

func New() *Service {
	service := &Service{
		clients: make(map[string]map[chan string]struct{}),
	}

	service.ServeHTTP = service.defaultServeHTTP
	return service
}

func (s *Service) defaultServeHTTP(topic string, writer http.ResponseWriter, request *http.Request) {
	// Set headers for server-sent events (SSE)
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	// Check if the ResponseWriter supports the Flusher interface
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create a channel for the client and register it
	msgCh := make(chan string, 10)
	s.registerClient(topic, msgCh)
	defer s.unregisterClient(topic, msgCh)

	// A comment/ping as per SSE spec - marks the start of the stream
	fmt.Fprintf(writer, ": ping\n\n")
	flusher.Flush()

	// Keep listening for messages to send to the client - keeps the connection open
	ctx := request.Context()
	log.Ctx(ctx).Debug().Msgf("Opened up SSE stream for topic: %s", topic)

	for {
		select {
		case msg := <-msgCh:
			fmt.Fprintf(writer, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ctx.Done():
			return
		}
	}
}

func (s *Service) registerClient(topic string, ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clients[topic]; !exists {
		s.clients[topic] = make(map[chan string]struct{})
	}
	s.clients[topic][ch] = struct{}{}
}

func (s *Service) unregisterClient(topic string, ch chan string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if clients, exists := s.clients[topic]; exists {
		delete(clients, ch)
		close(ch)
		if len(clients) == 0 {
			delete(s.clients, topic)
		}
	}
}

func (s *Service) Publish(ctx context.Context, topic string, msg string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for ch := range s.clients[topic] {
		select {
		case ch <- msg:
		default:
			log.Ctx(ctx).Warn().Msgf("client channel full, dropping message on topic %s", topic)
		}
	}
}
