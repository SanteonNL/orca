package pipeline

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHeaderRewriter_Transform(t *testing.T) {
	t.Run("replaces all occurrences of old string with new string", func(t *testing.T) {
		rewriter := ResponseHeaderRewriter{Old: "http://localhost:8080", New: "http://example.com"}
		headers := map[string][]string{
			"Location": {"http://localhost:8080/123"},
		}
		rewriter.Transform(nil, nil, headers)
		assert.Equal(t, "http://example.com/123", headers["Location"][0])
	})
}

func TestResponseBodyRewriter_Transform(t *testing.T) {
	t.Run("replaced all occurrences", func(t *testing.T) {
		expected := []byte("The quick brown fox (and another cat) jumps over the lazy cat")
		input := []byte("The quick brown fox (and another dog) jumps over the lazy dog")
		rewriter := ResponseBodyRewriter{
			Old: []byte("dog"),
			New: []byte("cat"),
		}
		rewriter.Transform(nil, &input, nil)
		assert.Equal(t, expected, input)
	})
	t.Run("nothing to replace", func(t *testing.T) {
		expected := []byte("The quick brown fox jumps over the lazy dog")
		input := []byte("The quick brown fox jumps over the lazy dog")
		rewriter := ResponseBodyRewriter{
			Old: []byte("cat"),
			New: []byte("dog"),
		}
		rewriter.Transform(nil, &input, nil)
		assert.Equal(t, expected, input)
	})
}
