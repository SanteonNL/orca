package to

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValue(t *testing.T) {
	t.Run("nil string", func(t *testing.T) {
		assert.Equal(t, "", Value((*string)(nil)))
	})
	t.Run("string", func(t *testing.T) {
		v := "hello"
		assert.Equal(t, "hello", Value(&v))
	})
}
