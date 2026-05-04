package otel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestError(t *testing.T) {
	newSpan := func() trace.ReadWriteSpan {
		tp := trace.NewTracerProvider()
		_, span := tp.Tracer("test").Start(nil, "test-span") //nolint:staticcheck
		return span.(trace.ReadWriteSpan)
	}

	t.Run("nil error returns nil", func(t *testing.T) {
		span := newSpan()
		result := Error(span, nil)
		assert.Nil(t, result)
	})
	t.Run("records error with default message", func(t *testing.T) {
		span := newSpan()
		err := errors.New("something went wrong")
		result := Error(span, err)
		require.Equal(t, err, result)
	})
	t.Run("records error with custom message", func(t *testing.T) {
		span := newSpan()
		err := errors.New("underlying error")
		result := Error(span, err, "custom message")
		require.Equal(t, err, result)
	})
	t.Run("records error with multiple messages", func(t *testing.T) {
		span := newSpan()
		err := errors.New("underlying error")
		result := Error(span, err, "part1", "part2")
		require.Equal(t, err, result)
	})
}
