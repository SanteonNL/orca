package events

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

var _ Type = StringEvent{}

type StringEvent struct {
	Value string
}

func (s StringEvent) Entity() messaging.Entity {
	return messaging.Entity{
		Name: "string-event",
	}
}

func (s StringEvent) Instance() Type {
	return &StringEvent{}
}

func TestDefaultManager_HasSubscribers(t *testing.T) {
	t.Run("returns false when no subscribers", func(t *testing.T) {
		manager := NewManager(messaging.NewMemoryBroker())
		assert.False(t, manager.HasSubscribers(StringEvent{}))
	})
	t.Run("returns true after subscribing", func(t *testing.T) {
		manager := NewManager(messaging.NewMemoryBroker())
		_ = manager.Subscribe(StringEvent{}, func(_ context.Context, _ Type) error { return nil })
		assert.True(t, manager.HasSubscribers(StringEvent{}))
	})
}

func TestInMemoryManager(t *testing.T) {
	t.Run("multiple subscribers", func(t *testing.T) {
		t.Run("both succeed", func(t *testing.T) {
			manager := NewManager(messaging.NewMemoryBroker())
			firstCalled := false
			secondCalled := false
			var capturedEvents []StringEvent
			err := manager.Subscribe(StringEvent{}, func(_ context.Context, event Type) error {
				capturedEvents = append(capturedEvents, *event.(*StringEvent))
				firstCalled = true
				return nil
			})
			require.NoError(t, err)
			err = manager.Subscribe(StringEvent{}, func(_ context.Context, event Type) error {
				capturedEvents = append(capturedEvents, *event.(*StringEvent))
				secondCalled = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), StringEvent{"test"})
			require.NoError(t, err)

			require.True(t, firstCalled)
			require.True(t, secondCalled)
			require.Len(t, capturedEvents, 2)
			require.Equal(t, StringEvent{"test"}, capturedEvents[0])
			require.Equal(t, StringEvent{"test"}, capturedEvents[1])
		})
		t.Run("first fails, second subscriber is still notified", func(t *testing.T) {
			manager := NewManager(messaging.NewMemoryBroker())
			secondCalled := false
			err := manager.Subscribe(StringEvent{}, func(_ context.Context, event Type) error {
				return errors.New("failed")
			})
			require.NoError(t, err)
			err = manager.Subscribe(StringEvent{}, func(_ context.Context, event Type) error {
				secondCalled = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), StringEvent{"test"})
			require.NoError(t, err)

			require.True(t, secondCalled)
		})
	})
}
