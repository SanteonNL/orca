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

func TestNilString(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		assert.Nil(t, NilString(""))
	})
	t.Run("non-empty string", func(t *testing.T) {
		assert.NotNil(t, NilString("hello"))
	})
}
