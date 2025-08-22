package debug

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetCallerName(t *testing.T) {
	t.Run("DirectCall", func(t *testing.T) {
		assert.Equal(t, "func1", GetCallerName())
	})

	t.Run("WithSkip0", func(t *testing.T) {
		assert.Equal(t, "GetCallerName", GetCallerName(0))
	})

	t.Run("WithSkip1", func(t *testing.T) {
		assert.Equal(t, "func3", GetCallerName(1))
	})

	t.Run("NestedCall", func(t *testing.T) {
		assert.Equal(t, "helperFunction", helperFunction())
	})

	t.Run("DeepNestedCall", func(t *testing.T) {
		assert.Equal(t, "intermediateFunction", deepHelperFunction())
	})

	t.Run("ExcessiveSkip", func(t *testing.T) {
		assert.Equal(t, "unknown", GetCallerName(100))
	})

	assert.Equal(t, "TestGetCallerName", GetCallerName())
	assert.Equal(t, "callGetCallerName", (&testCaller{}).callGetCallerName())
}

// Helper functions for testing nested calls
func helperFunction() string {
	return GetCallerName()
}

func deepHelperFunction() string {
	return intermediateFunction()
}

func intermediateFunction() string {
	return GetCallerName()
}

type testCaller struct{}

func (tc *testCaller) callGetCallerName() string {
	return GetCallerName()
}
