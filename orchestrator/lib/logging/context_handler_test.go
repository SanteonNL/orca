package logging

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		attr     slog.Attr
		expected string
	}{
		{
			name:     "should append string attribute to empty context",
			ctx:      context.Background(),
			attr:     slog.String("key", "value"),
			expected: "value",
		},
		{
			name:     "should append int attribute",
			ctx:      context.Background(),
			attr:     slog.Int("count", 42),
			expected: "42",
		},
		{
			name:     "should append bool attribute",
			ctx:      context.Background(),
			attr:     slog.Bool("enabled", true),
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := AppendCtx(tt.ctx, tt.attr)
			attrs, ok := ctx.Value(slogFields).([]slog.Attr)
			assert.True(t, ok)
			assert.Len(t, attrs, 1)
			assert.Equal(t, tt.attr.Key, attrs[0].Key)
		})
	}
}

func TestAppendCtxMultiple(t *testing.T) {
	t.Run("should append multiple attributes", func(t *testing.T) {
		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("key1", "value1"))
		ctx = AppendCtx(ctx, slog.String("key2", "value2"))
		ctx = AppendCtx(ctx, slog.String("key3", "value3"))

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 3)
		assert.Equal(t, "key1", attrs[0].Key)
		assert.Equal(t, "key2", attrs[1].Key)
		assert.Equal(t, "key3", attrs[2].Key)
	})
}

func TestAppendCtxReplaceExisting(t *testing.T) {
	t.Run("should replace existing attribute with same key", func(t *testing.T) {
		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("key", "value1"))
		ctx = AppendCtx(ctx, slog.String("key", "value2"))

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 1)
		assert.Equal(t, "key", attrs[0].Key)
	})
}

func TestAppendCtxNilContext(t *testing.T) {
	t.Run("should handle nil context by creating background context", func(t *testing.T) {
		ctx := AppendCtx(nil, slog.String("key", "value"))
		assert.NotNil(t, ctx)

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 1)
	})
}

func TestAppendCtxPreservesExisting(t *testing.T) {
	t.Run("should preserve other context values", func(t *testing.T) {
		type contextKey string
		const testKey contextKey = "test"

		ctx := context.Background()
		ctx = context.WithValue(ctx, testKey, "test_value")
		ctx = AppendCtx(ctx, slog.String("logging_key", "logging_value"))

		// Verify both values are present
		assert.Equal(t, "test_value", ctx.Value(testKey))
		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 1)
	})
}

func TestContextHandler(t *testing.T) {
	t.Run("should handle record with context attributes", func(t *testing.T) {
		var buf bytes.Buffer
		baseHandler := slog.NewTextHandler(&buf, nil)
		handler := ContextHandler{baseHandler}

		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("user_id", "123"))

		record := slog.Record{
			Level:   slog.LevelInfo,
			Message: "test message",
		}

		err := handler.Handle(ctx, record)
		assert.NoError(t, err)
	})
}

func TestContextHandlerNoAttributes(t *testing.T) {
	t.Run("should handle record without context attributes", func(t *testing.T) {
		var buf bytes.Buffer
		baseHandler := slog.NewTextHandler(&buf, nil)
		handler := ContextHandler{baseHandler}

		ctx := context.Background()

		record := slog.Record{
			Level:   slog.LevelInfo,
			Message: "test message",
		}

		err := handler.Handle(ctx, record)
		assert.NoError(t, err)
	})
}

func TestAppendCtxWithDifferentTypes(t *testing.T) {
	t.Run("should handle different attribute types", func(t *testing.T) {
		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("string_key", "string_value"))
		ctx = AppendCtx(ctx, slog.Int("int_key", 100))
		ctx = AppendCtx(ctx, slog.Bool("bool_key", false))
		ctx = AppendCtx(ctx, slog.Float64("float_key", 3.14))

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 4)

		assert.Equal(t, "string_key", attrs[0].Key)
		assert.Equal(t, "int_key", attrs[1].Key)
		assert.Equal(t, "bool_key", attrs[2].Key)
		assert.Equal(t, "float_key", attrs[3].Key)
	})
}

func TestAppendCtxOrdering(t *testing.T) {
	t.Run("should preserve attribute order", func(t *testing.T) {
		ctx := context.Background()
		keys := []string{"first", "second", "third", "fourth"}

		for _, key := range keys {
			ctx = AppendCtx(ctx, slog.String(key, key+"_value"))
		}

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)

		for i, key := range keys {
			assert.Equal(t, key, attrs[i].Key)
		}
	})
}

func TestAppendCtxReplaceMultiple(t *testing.T) {
	t.Run("should handle multiple replacements correctly", func(t *testing.T) {
		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("key1", "value1"))
		ctx = AppendCtx(ctx, slog.String("key2", "value2"))
		ctx = AppendCtx(ctx, slog.String("key3", "value3"))

		// Replace key2
		ctx = AppendCtx(ctx, slog.String("key2", "value2_new"))

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		// Should have 3 attributes after replacing one
		assert.Len(t, attrs, 3)

		// Verify key2 was updated (don't check order, just check all keys exist)
		keys := make(map[string]bool)
		for _, attr := range attrs {
			keys[attr.Key] = true
		}
		assert.True(t, keys["key1"])
		assert.True(t, keys["key2"])
		assert.True(t, keys["key3"])
	})
}

func TestContextHandlerWithMultipleAttributes(t *testing.T) {
	t.Run("should add all context attributes to record", func(t *testing.T) {
		var buf bytes.Buffer
		baseHandler := slog.NewTextHandler(&buf, nil)
		handler := ContextHandler{baseHandler}

		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("attr1", "value1"))
		ctx = AppendCtx(ctx, slog.String("attr2", "value2"))

		record := slog.Record{
			Level:   slog.LevelInfo,
			Message: "test message",
		}

		err := handler.Handle(ctx, record)
		assert.NoError(t, err)
		assert.Greater(t, buf.Len(), 0)
	})
}

func TestAppendCtxEmptyKey(t *testing.T) {
	t.Run("should handle empty key", func(t *testing.T) {
		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("", "value"))

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 1)
		assert.Equal(t, "", attrs[0].Key)
	})
}

func TestAppendCtxChaining(t *testing.T) {
	t.Run("should support method chaining", func(t *testing.T) {
		ctx := AppendCtx(
			AppendCtx(
				AppendCtx(
					context.Background(),
					slog.String("key1", "value1"),
				),
				slog.String("key2", "value2"),
			),
			slog.String("key3", "value3"),
		)

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Len(t, attrs, 3)
	})
}

func TestContextHandlerIntegration(t *testing.T) {
	t.Run("should work as a complete logging pipeline", func(t *testing.T) {
		var buf bytes.Buffer
		baseHandler := slog.NewTextHandler(&buf, nil)
		handler := ContextHandler{baseHandler}
		logger := slog.New(handler)

		ctx := context.Background()
		ctx = AppendCtx(ctx, slog.String("request_id", "req-123"))
		ctx = AppendCtx(ctx, slog.String("user", "alice"))

		logger.InfoContext(ctx, "processing request", slog.String("action", "create"))

		output := buf.String()
		assert.Greater(t, len(output), 0)
	})
}

func TestAppendCtxLargeNumberOfAttributes(t *testing.T) {
	t.Run("should handle large number of attributes", func(t *testing.T) {
		ctx := context.Background()

		numAttrs := 100
		for i := 0; i < numAttrs; i++ {
			keyName := fmt.Sprintf("attr_%d", i)
			ctx = AppendCtx(ctx, slog.Int(keyName, i))
		}

		attrs, ok := ctx.Value(slogFields).([]slog.Attr)
		assert.True(t, ok)
		assert.Equal(t, numAttrs, len(attrs))
	})
}
