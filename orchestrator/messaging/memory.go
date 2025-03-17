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

func (m *MemoryBroker) Receive(queue Topic, handler func(context.Context, Message) error) error {
	m.handlers[queue.Name] = append(m.handlers[queue.Name], handler)
	return nil
}

func (m *MemoryBroker) SendMessage(ctx context.Context, topic Topic, message *Message) error {
	if len(m.handlers[topic.Name]) == 0 {
		return fmt.Errorf("no handlers for topic %s", topic.Name)
	}
	for _, handler := range m.handlers[topic.Name] {
		if err := handler(ctx, *message); err != nil {
			m.LastHandlerError.Store(&err)
			log.Ctx(ctx).Warn().Msgf("Handler for topic %s failed: %s", topic.Name, err.Error())
		}
	}
	return nil
}

func (m *MemoryBroker) Close(_ context.Context) error {
	m.handlers = map[string][]func(context.Context, Message) error{}
	return nil
}
