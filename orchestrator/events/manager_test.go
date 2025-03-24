package events

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/stretchr/testify/require"
	"testing"
)

var _ Type = StringEvent{}

type StringEvent struct {
	Value string
}

func (s StringEvent) Topic() messaging.Topic {
	return messaging.Topic{
		Name: "string-event",
	}
}

func (s StringEvent) Instance() Type {
	return &StringEvent{}
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
