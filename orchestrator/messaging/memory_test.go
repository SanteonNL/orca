package messaging

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryBroker_ReceiveFromQueue(t *testing.T) {
	broker := NewMemoryBroker()
	called := false
	err := broker.ReceiveFromQueue(Entity{Name: "test-queue"}, func(_ context.Context, _ Message) error {
		called = true
		return nil
	})
	require.NoError(t, err)

	_ = broker.SendMessage(context.Background(), Entity{Name: "test-queue"}, &Message{Body: []byte("hello")})
	assert.True(t, called)
}

func TestMemoryBroker_SendMessage(t *testing.T) {
	t.Run("no handlers returns error", func(t *testing.T) {
		broker := NewMemoryBroker()
		err := broker.SendMessage(context.Background(), Entity{Name: "no-handlers"}, &Message{Body: []byte("data")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no handlers for entity no-handlers")
	})
	t.Run("handler error is stored", func(t *testing.T) {
		broker := NewMemoryBroker()
		handlerErr := errors.New("handler failed")
		_ = broker.ReceiveFromQueue(Entity{Name: "q"}, func(_ context.Context, _ Message) error {
			return handlerErr
		})
		err := broker.SendMessage(context.Background(), Entity{Name: "q"}, &Message{Body: []byte("data")})
		require.NoError(t, err)
		stored := broker.LastHandlerError.Load()
		require.NotNil(t, stored)
		assert.Equal(t, handlerErr, *stored)
	})
}

func TestMemoryBroker_Close(t *testing.T) {
	broker := NewMemoryBroker()
	_ = broker.ReceiveFromQueue(Entity{Name: "q"}, func(_ context.Context, _ Message) error { return nil })
	err := broker.Close(context.Background())
	require.NoError(t, err)
	// After close, handlers are cleared
	sendErr := broker.SendMessage(context.Background(), Entity{Name: "q"}, &Message{Body: []byte("data")})
	assert.Error(t, sendErr)
}
