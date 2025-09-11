package debug

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetFullCallerName(t *testing.T) {
	t.Run("DirectCall", func(t *testing.T) {
		assert.Equal(t, "debug.TestGetFullCallerName.func1", GetFullCallerName())
	})

	t.Run("WithSkip0", func(t *testing.T) {
		assert.Equal(t, "debug.GetFullCallerName", GetFullCallerName(0))
	})

	t.Run("WithSkip1", func(t *testing.T) {
		assert.Equal(t, "debug.TestGetFullCallerName.func3", GetFullCallerName(1))
	})

	t.Run("NestedCall", func(t *testing.T) {
		assert.Equal(t, "debug.helperFunction", helperFunction())
	})

	t.Run("DeepNestedCall", func(t *testing.T) {
		assert.Equal(t, "debug.intermediateFunction", deepHelperFunction())
	})

	t.Run("ExcessiveSkip", func(t *testing.T) {
		assert.Equal(t, "unknown", GetFullCallerName(100))
	})

	assert.Equal(t, "debug.TestGetFullCallerName", GetFullCallerName())
	assert.Equal(t, "debug.(*testCaller).callGetCallerName", (&testCaller{}).callGetCallerName())
}

// Helper functions for testing nested calls
func helperFunction() string {
	return GetFullCallerName()
}

func deepHelperFunction() string {
	return intermediateFunction()
}

func intermediateFunction() string {
	return GetFullCallerName()
}

type testCaller struct{}

func (tc *testCaller) callGetCallerName() string {
	return GetFullCallerName()
}
