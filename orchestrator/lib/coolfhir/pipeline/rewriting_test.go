package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestMetaSourceSetter_do(t *testing.T) {
	t.Run("sets meta.source", func(t *testing.T) {
		expected := `{"meta":{"source":"http://example.com"}}`
		input := []byte(`{}`)
		transformer := MetaSourceSetter{URI: "http://example.com"}
		err := transformer.do(&input)
		assert.NoError(t, err)
		assert.JSONEq(t, expected, string(input))
	})
	t.Run("overwrites existing meta.source", func(t *testing.T) {
		expected := `{"meta":{"source":"http://example.com"}}`
		input := []byte(`{"meta":{"source":"http://localhost:8080"}}`)
		transformer := MetaSourceSetter{URI: "http://example.com"}
		err := transformer.do(&input)
		assert.NoError(t, err)
		assert.JSONEq(t, expected, string(input))
	})
	t.Run("invalid resource", func(t *testing.T) {
		input := []byte(`invalid`)
		transformer := MetaSourceSetter{URI: "http://example.com"}
		err := transformer.do(&input)
		assert.Error(t, err)
	})
}

func TestStripResponseHeaders_Transform(t *testing.T) {
	t.Run("keeps allowed headers and removes disallowed ones", func(t *testing.T) {
		headers := map[string][]string{
			"Content-Type":    {"application/fhir+json"},
			"Content-Length":  {"1234"},
			"ETag":            {"W/\"1\""},
			"Location":        {"http://example.com/Patient/123"},
			"User-Agent":      {"MyServer/1.0"},
			"Server":          {"nginx/1.18.0"},
			"X-Powered-By":    {"Express"},
			"Cache-Control":   {"no-cache"},
			"X-Custom-Header": {"should-be-removed"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should keep allowed headers
		assert.Contains(t, headers, "Content-Type")
		assert.Contains(t, headers, "Content-Length")
		assert.Contains(t, headers, "ETag")
		assert.Contains(t, headers, "Location")
		assert.Contains(t, headers, "Cache-Control")

		// Should remove disallowed headers
		assert.NotContains(t, headers, "User-Agent")
		assert.NotContains(t, headers, "Server")
		assert.NotContains(t, headers, "X-Powered-By")
		assert.NotContains(t, headers, "X-Custom-Header")

		// Verify values remain unchanged for allowed headers
		assert.Equal(t, "application/fhir+json", headers["Content-Type"][0])
		assert.Equal(t, "1234", headers["Content-Length"][0])
		assert.Equal(t, "W/\"1\"", headers["ETag"][0])
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		headers := map[string][]string{
			"content-type":   {"application/fhir+json"},
			"CONTENT-LENGTH": {"1234"},
			"etag":           {"W/\"1\""},
			"LOCATION":       {"http://example.com/Patient/123"},
			"user-agent":     {"MyServer/1.0"},
			"SERVER":         {"nginx/1.18.0"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should keep allowed headers regardless of case
		assert.Contains(t, headers, "content-type")
		assert.Contains(t, headers, "CONTENT-LENGTH")
		assert.Contains(t, headers, "etag")
		assert.Contains(t, headers, "LOCATION")

		// Should remove disallowed headers regardless of case
		assert.NotContains(t, headers, "user-agent")
		assert.NotContains(t, headers, "SERVER")
	})

	t.Run("all FHIR headers are preserved", func(t *testing.T) {
		headers := map[string][]string{
			"Content-Type":   {"application/fhir+json"},
			"Content-Length": {"1234"},
			"Location":       {"http://example.com/Patient/123"},
			"ETag":           {"W/\"1\""},
			"Last-Modified":  {"Mon, 20 Nov 2023 10:00:00 GMT"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// All FHIR headers should be preserved
		assert.Len(t, headers, 5)
		assert.Contains(t, headers, "Content-Type")
		assert.Contains(t, headers, "Content-Length")
		assert.Contains(t, headers, "Location")
		assert.Contains(t, headers, "ETag")
		assert.Contains(t, headers, "Last-Modified")
	})

	t.Run("CORS headers are preserved", func(t *testing.T) {
		headers := map[string][]string{
			"Access-Control-Allow-Origin":  {"*"},
			"Access-Control-Allow-Methods": {"GET, POST, PUT"},
			"Access-Control-Allow-Headers": {"Content-Type, Authorization"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// All CORS headers should be preserved
		assert.Len(t, headers, 3)
		assert.Contains(t, headers, "Access-Control-Allow-Origin")
		assert.Contains(t, headers, "Access-Control-Allow-Methods")
		assert.Contains(t, headers, "Access-Control-Allow-Headers")
	})

	t.Run("security headers are preserved", func(t *testing.T) {
		headers := map[string][]string{
			"X-Content-Type-Options":  {"nosniff"},
			"X-Frame-Options":         {"DENY"},
			"Content-Security-Policy": {"default-src 'self'"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// All security headers should be preserved
		assert.Len(t, headers, 3)
		assert.Contains(t, headers, "X-Content-Type-Options")
		assert.Contains(t, headers, "X-Frame-Options")
		assert.Contains(t, headers, "Content-Security-Policy")
	})

	t.Run("caching headers are preserved", func(t *testing.T) {
		headers := map[string][]string{
			"Cache-Control": {"no-cache, no-store"},
			"Expires":       {"Mon, 20 Nov 2023 10:00:00 GMT"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// All caching headers should be preserved
		assert.Len(t, headers, 2)
		assert.Contains(t, headers, "Cache-Control")
		assert.Contains(t, headers, "Expires")
	})

	t.Run("empty headers map", func(t *testing.T) {
		headers := map[string][]string{}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Headers should remain empty
		assert.Len(t, headers, 0)
	})

	t.Run("nil headers map", func(t *testing.T) {
		var headers map[string][]string

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)
	})

	t.Run("headers with empty values", func(t *testing.T) {
		headers := map[string][]string{
			"Content-Type": {},
			"User-Agent":   {},
			"Server":       {""},
			"X-Powered-By": {"", ""},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should keep allowed headers even with empty values
		assert.Contains(t, headers, "Content-Type")
		assert.Empty(t, headers["Content-Type"])

		// Should remove disallowed headers even with empty values
		assert.NotContains(t, headers, "User-Agent")
		assert.NotContains(t, headers, "Server")
		assert.NotContains(t, headers, "X-Powered-By")
	})

	t.Run("only disallowed headers", func(t *testing.T) {
		headers := map[string][]string{
			"User-Agent":      {"MyServer/1.0"},
			"Server":          {"nginx/1.18.0"},
			"X-Powered-By":    {"Express"},
			"X-Custom-Header": {"custom-value"},
			"Via":             {"1.1 proxy"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// All headers should be removed
		assert.Len(t, headers, 0)
	})

	t.Run("headers with multiple values", func(t *testing.T) {
		headers := map[string][]string{
			"Content-Type": {"application/json", "charset=utf-8"},
			"User-Agent":   {"MyServer/1.0", "ExtraInfo"},
			"Server":       {"nginx/1.18.0", "Ubuntu"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should keep allowed headers with multiple values
		assert.Contains(t, headers, "Content-Type")
		assert.Len(t, headers["Content-Type"], 2)
		assert.Equal(t, "application/json", headers["Content-Type"][0])
		assert.Equal(t, "charset=utf-8", headers["Content-Type"][1])

		// Should remove disallowed headers regardless of multiple values
		assert.NotContains(t, headers, "User-Agent")
		assert.NotContains(t, headers, "Server")
	})

	t.Run("edge case - header names with special characters", func(t *testing.T) {
		headers := map[string][]string{
			"Content-Type":    {"application/fhir+json"},
			"X-Custom-Header": {"should-be-removed"},
			"X_Underscore":    {"should-be-removed"},
			"X.Dot.Header":    {"should-be-removed"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should keep allowed headers
		assert.Contains(t, headers, "Content-Type")

		// Should remove all custom headers with special characters
		assert.NotContains(t, headers, "X-Custom-Header")
		assert.NotContains(t, headers, "X_Underscore")
		assert.NotContains(t, headers, "X.Dot.Header")
	})

	t.Run("edge case - mixed case in allowlist matching", func(t *testing.T) {
		headers := map[string][]string{
			"content-TYPE":   {"application/fhir+json"},
			"Content-LENGTH": {"1234"},
			"eTaG":           {"W/\"1\""},
			"user-AGENT":     {"MyServer/1.0"},
		}

		transformer := StripResponseHeaders{}
		transformer.Transform(nil, nil, headers)

		// Should handle mixed case correctly for allowed headers
		assert.Contains(t, headers, "content-TYPE")
		assert.Contains(t, headers, "Content-LENGTH")
		assert.Contains(t, headers, "eTaG")

		// Should remove disallowed headers regardless of case
		assert.NotContains(t, headers, "user-AGENT")
	})
}
