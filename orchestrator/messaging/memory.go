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

func (m *MemoryBroker) Receive(queue string, handler func(context.Context, Message) error) error {
	m.handlers[queue] = append(m.handlers[queue], handler)
	return nil
}

func (m *MemoryBroker) SendMessage(ctx context.Context, topic string, message *Message) error {
	if len(m.handlers[topic]) == 0 {
		return fmt.Errorf("no handlers for topic %s", topic)
	}
	for _, handler := range m.handlers[topic] {
		if err := handler(ctx, *message); err != nil {
			m.LastHandlerError.Store(&err)
			log.Ctx(ctx).Warn().Msgf("Handler for topic %s failed: %s", topic, err.Error())
		}
	}
	return nil
}

func (m *MemoryBroker) Close(_ context.Context) error {
	m.handlers = map[string][]func(context.Context, Message) error{}
	return nil
}
