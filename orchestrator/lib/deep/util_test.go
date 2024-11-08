package deep

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEqual(t *testing.T) {
	t.Run("Maps", func(t *testing.T) {
		t.Run("Equal", func(t *testing.T) {
			a := map[string]interface{}{
				"foo": "bar",
			}
			b := map[string]interface{}{
				"foo": "bar",
			}
			require.True(t, Equal(a, b))
		})
		t.Run("NotEqual", func(t *testing.T) {
			a := map[string]interface{}{
				"foo": "bar",
			}
			b := map[string]interface{}{
				"foo": "baz",
			}
			require.False(t, Equal(a, b))
		})
	})
	t.Run("Slices", func(t *testing.T) {
		t.Run("Equal", func(t *testing.T) {
			a := []interface{}{"foo", "bar"}
			b := []interface{}{"foo", "bar"}
			require.True(t, Equal(a, b))
		})
		t.Run("NotEqual", func(t *testing.T) {
			a := []interface{}{"foo", "bar"}
			b := []interface{}{"foo", "baz"}
			require.False(t, Equal(a, b))
		})
	})
	t.Run("nil", func(t *testing.T) {
		t.Run("both nil", func(t *testing.T) {
			require.True(t, Equal(nil, nil))
		})
		t.Run("one nil", func(t *testing.T) {
			require.False(t, Equal(nil, "foo"))
		})
		t.Run("other nil", func(t *testing.T) {
			require.False(t, Equal("foo", nil))
		})
	})
}
