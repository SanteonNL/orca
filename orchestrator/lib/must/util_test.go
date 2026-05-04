package must

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURL(t *testing.T) {
	t.Run("valid URL", func(t *testing.T) {
		u := ParseURL("https://example.com/path")
		require.NotNil(t, u)
		assert.Equal(t, "https", u.Scheme)
		assert.Equal(t, "example.com", u.Host)
		assert.Equal(t, "/path", u.Path)
	})
	t.Run("panics on invalid URL", func(t *testing.T) {
		assert.Panics(t, func() {
			ParseURL("://invalid")
		})
	})
}

func TestMarshalJSON(t *testing.T) {
	t.Run("marshals valid value", func(t *testing.T) {
		result := MarshalJSON(map[string]string{"key": "value"})
		assert.JSONEq(t, `{"key":"value"}`, string(result))
	})
	t.Run("panics on unmarshalable value", func(t *testing.T) {
		assert.Panics(t, func() {
			MarshalJSON(make(chan int))
		})
	})
}
