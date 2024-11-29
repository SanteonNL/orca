package slices

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDeduplicate(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		var in []int = nil
		got := Deduplicate(in, func(a, b int) bool {
			return a == b
		})
		assert.Nil(t, got)
	})
	t.Run("deduplicate", func(t *testing.T) {
		in := []int{1, 2, 3, 2, 1}
		got := Deduplicate(in, func(a, b int) bool {
			return a == b
		})
		want := []int{1, 2, 3}
		assert.Equal(t, want, got)
	})
	t.Run("nothing to deduplicate", func(t *testing.T) {
		in := []int{1, 2, 3}
		got := Deduplicate(in, func(a, b int) bool {
			return a == b
		})
		assert.Equal(t, in, got)
	})
}
