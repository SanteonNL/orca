package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "careplancontributor"

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
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		request.Context(),
		"SSE.ServeHTTP",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("operation.name", "SSE/ServeHTTP"),
			attribute.String("sse.topic", topic),
			attribute.String("http.method", request.Method),
			attribute.String("http.url", request.URL.String()),
		),
	)
	defer span.End()

	// Set headers for server-sent events (SSE)
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	// Check if the ResponseWriter supports the Flusher interface
	flusher, ok := writer.(http.Flusher)
	if !ok {
		span.RecordError(fmt.Errorf("streaming unsupported"))
		span.SetStatus(codes.Error, "streaming unsupported")
		http.Error(writer, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Create a channel for the client and register it
	msgCh := make(chan string, 10)
	s.registerClient(ctx, topic, msgCh)
	defer s.unregisterClient(ctx, topic, msgCh)

	// A comment/ping as per SSE spec - marks the start of the stream
	fmt.Fprintf(writer, ": ping\n\n")
	flusher.Flush()

	// Keep listening for messages to send to the client - keeps the connection open
	log.Ctx(ctx).Debug().Msgf("Opened up SSE stream for topic: %s", topic)
	span.AddEvent("sse.stream.opened", trace.WithAttributes(
		attribute.String("sse.topic", topic),
	))

	messageCount := 0
	for {
		select {
		case msg := <-msgCh:
			messageCount++
			fmt.Fprintf(writer, "data: %s\n\n", msg)
			flusher.Flush()
			span.AddEvent("sse.message.sent", trace.WithAttributes(
				attribute.Int("message.count", messageCount),
				attribute.Int("message.size", len(msg)),
			))
		case <-ctx.Done():
			span.SetAttributes(
				attribute.Int("sse.messages_sent", messageCount),
			)
			span.AddEvent("sse.stream.closed", trace.WithAttributes(
				attribute.String("close.reason", "context_done"),
			))
			return
		}
	}
}

func (s *Service) registerClient(ctx context.Context, topic string, ch chan string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"SSE.registerClient",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("operation.name", "SSE/registerClient"),
			attribute.String("sse.topic", topic),
		),
	)
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	clientCountBefore := 0
	if clients, exists := s.clients[topic]; exists {
		clientCountBefore = len(clients)
	}

	if _, exists := s.clients[topic]; !exists {
		s.clients[topic] = make(map[chan string]struct{})
	}
	s.clients[topic][ch] = struct{}{}

	clientCountAfter := len(s.clients[topic])
	span.SetAttributes(
		attribute.Int("sse.clients_before", clientCountBefore),
		attribute.Int("sse.clients_after", clientCountAfter),
		attribute.Bool("sse.topic_created", clientCountBefore == 0),
	)
}

func (s *Service) unregisterClient(ctx context.Context, topic string, ch chan string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"SSE.unregisterClient",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("operation.name", "SSE/unregisterClient"),
			attribute.String("sse.topic", topic),
		),
	)
	defer span.End()

	s.mu.Lock()
	defer s.mu.Unlock()

	clientCountBefore := 0
	topicExists := false
	if clients, exists := s.clients[topic]; exists {
		clientCountBefore = len(clients)
		topicExists = true
	}

	topicDeleted := false
	if clients, exists := s.clients[topic]; exists {
		delete(clients, ch)
		close(ch)
		if len(clients) == 0 {
			delete(s.clients, topic)
			topicDeleted = true
		}
	}

	clientCountAfter := 0
	if clients, exists := s.clients[topic]; exists {
		clientCountAfter = len(clients)
	}

	span.SetAttributes(
		attribute.Int("sse.clients_before", clientCountBefore),
		attribute.Int("sse.clients_after", clientCountAfter),
		attribute.Bool("sse.topic_existed", topicExists),
		attribute.Bool("sse.topic_deleted", topicDeleted),
	)
}

func (s *Service) Publish(ctx context.Context, topic string, msg string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"SSE.Publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("operation.name", "SSE/Publish"),
			attribute.String("sse.topic", topic),
			attribute.Int("message.size", len(msg)),
		),
	)
	defer span.End()

	s.mu.RLock()
	defer s.mu.RUnlock()

	clientCount := len(s.clients[topic])
	messagesDelivered := 0
	messagesDropped := 0

	span.SetAttributes(
		attribute.Int("sse.client_count", clientCount),
	)

	for ch := range s.clients[topic] {
		select {
		case ch <- msg:
			messagesDelivered++
		default:
			messagesDropped++
			log.Ctx(ctx).Warn().Msgf("client channel full, dropping message on topic %s", topic)
		}
	}

	span.SetAttributes(
		attribute.Int("sse.messages_delivered", messagesDelivered),
		attribute.Int("sse.messages_dropped", messagesDropped),
	)

	if messagesDropped > 0 {
		span.AddEvent("sse.messages_dropped", trace.WithAttributes(
			attribute.Int("dropped_count", messagesDropped),
			attribute.String("reason", "client_channel_full"),
		))
	}
}
