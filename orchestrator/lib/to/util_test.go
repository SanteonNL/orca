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

func TestEmptyString(t *testing.T) {
	t.Run("nil returns empty string", func(t *testing.T) {
		assert.Equal(t, "", EmptyString(nil))
	})
	t.Run("non-nil returns value", func(t *testing.T) {
		v := "hello"
		assert.Equal(t, "hello", EmptyString(&v))
	})
}

func TestEmpty(t *testing.T) {
	t.Run("nil pointer returns zero value", func(t *testing.T) {
		assert.Equal(t, 0, Empty((*int)(nil)))
	})
	t.Run("non-nil pointer returns value", func(t *testing.T) {
		v := 42
		assert.Equal(t, 42, Empty(&v))
	})
}

func TestPtr(t *testing.T) {
	v := Ptr("hello")
	assert.Equal(t, "hello", *v)
}
