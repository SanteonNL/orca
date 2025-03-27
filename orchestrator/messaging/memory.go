package messaging

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
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

func (m *MemoryBroker) ReceiveFromTopic(topic Entity, handler func(context.Context, Message) error, subscriberName string) error {
	m.handlers[topic.Name] = append(m.handlers[topic.Name], handler)
	return nil
}

func (m *MemoryBroker) ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error {
	m.handlers[queue.Name] = append(m.handlers[queue.Name], handler)
	return nil
}

func (m *MemoryBroker) SendMessage(ctx context.Context, entity Entity, message *Message) error {
	if len(m.handlers[entity.Name]) == 0 {
		return fmt.Errorf("no handlers for entity %s", entity.Name)
	}
	for _, handler := range m.handlers[entity.Name] {
		if err := handler(ctx, *message); err != nil {
			m.LastHandlerError.Store(&err)
			log.Ctx(ctx).Warn().Msgf("Handler for entity %s failed: %s", entity.Name, err.Error())
		}
	}
	return nil
}

func (m *MemoryBroker) Close(_ context.Context) error {
	m.handlers = map[string][]func(context.Context, Message) error{}
	return nil
}
