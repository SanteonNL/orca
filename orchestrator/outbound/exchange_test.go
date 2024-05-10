package outbound

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_buildFHIRResourceURL(t *testing.T) {
	t.Run("base URL path is retained", func(t *testing.T) {
		result, err := buildFHIRResourceURL("https://example.com/fhir", "test/123")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/fhir/test/123", result)
	})
	t.Run("base path ends with slash", func(t *testing.T) {
		result, err := buildFHIRResourceURL("https://example.com/", "test/123")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/test/123", result)
	})
	t.Run("FHIR operation path starts with slash", func(t *testing.T) {
		result, err := buildFHIRResourceURL("https://example.com", "/test/123")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/test/123", result)
	})
	t.Run("both have slash", func(t *testing.T) {
		result, err := buildFHIRResourceURL("https://example.com/", "/test/123")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/test/123", result)
	})
	t.Run("query parameters are retained", func(t *testing.T) {
		result, err := buildFHIRResourceURL("https://example.com", "/test/123?foo=bar")
		require.NoError(t, err)
		assert.Equal(t, "https://example.com/test/123?foo=bar", result)
	})
}
