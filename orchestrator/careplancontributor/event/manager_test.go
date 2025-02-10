package events

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInMemoryManager(t *testing.T) {
	t.Run("multiple subscribers", func(t *testing.T) {
		t.Run("both succeed", func(t *testing.T) {
			manager := NewInMemoryManager()
			firstCalled := false
			secondCalled := false
			err := manager.Subscribe("Task", "test", func(_ context.Context, event Instance) error {
				firstCalled = true
				return nil
			})
			require.NoError(t, err)
			err = manager.Subscribe("Task", "test", func(_ context.Context, event Instance) error {
				secondCalled = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), "Task", Instance{})
			require.NoError(t, err)

			require.True(t, firstCalled)
			require.True(t, secondCalled)
		})
		t.Run("first fails, second subscriber is still notified", func(t *testing.T) {
			manager := NewInMemoryManager()
			secondCalled := false
			err := manager.Subscribe("Task", "test", func(_ context.Context, event Instance) error {
				return errors.New("failed")
			})
			require.NoError(t, err)
			err = manager.Subscribe("Task", "test", func(_ context.Context, event Instance) error {
				secondCalled = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), "Task", Instance{})
			require.NoError(t, err)

			require.True(t, secondCalled)
		})
	})
	t.Run("wildcard subscriber", func(t *testing.T) {
		t.Run("empty type", func(t *testing.T) {
			manager := NewInMemoryManager()
			called := false
			err := manager.Subscribe("", "test", func(_ context.Context, event Instance) error {
				called = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), "Task", Instance{})
			require.NoError(t, err)
			require.True(t, called)
		})
		t.Run("asterisk type", func(t *testing.T) {
			manager := NewInMemoryManager()
			called := false
			err := manager.Subscribe("*", "test", func(_ context.Context, event Instance) error {
				called = true
				return nil
			})
			require.NoError(t, err)

			err = manager.Notify(context.Background(), "Task", Instance{})
			require.NoError(t, err)
			require.True(t, called)
		})
	})
}
