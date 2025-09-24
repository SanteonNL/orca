package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
)

var _ Broker = &MemoryBroker{}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{
		handlers: make(map[string][]func(context.Context, Message) error),
	}
}

type MemoryBroker struct {
	handlers         map[string][]func(context.Context, Message) error
	LastHandlerError atomic.Pointer[error]
}

func (m *MemoryBroker) ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error {
	m.handlers[queue.Name] = append(m.handlers[queue.Name], handler)
	return nil
}

func (m *MemoryBroker) SendMessage(_ context.Context, entity Entity, message *Message) error {
	if len(m.handlers[entity.Name]) == 0 {
		return fmt.Errorf("no handlers for entity %s", entity.Name)
	}
	// Create a new context for the handlers, because it is supposed to be an asynchronous (background) operation
	ctx := context.Background()
	for _, handler := range m.handlers[entity.Name] {
		if err := handler(ctx, *message); err != nil {
			m.LastHandlerError.Store(&err)
			slog.WarnContext(ctx, "Handler for entity failed",
				slog.String("entity_name", entity.Name),
				slog.String("error", err.Error()),
			)
		}
	}
	return nil
}

func (m *MemoryBroker) Close(_ context.Context) error {
	m.handlers = map[string][]func(context.Context, Message) error{}
	return nil
}
