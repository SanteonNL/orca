package pipeline

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

var _ HttpResponseTransformer = &MetaSourceSetter{}

// MetaSourceSetter is a transformer that sets (or overwrites) the meta.source field in a FHIR resource.
type MetaSourceSetter struct {
	URI string
}

func (m MetaSourceSetter) Transform(_ *int, responseBody *[]byte, _ map[string][]string) {
	if err := m.do(responseBody); err != nil {
		slog.Error("MetaSourceSetter: failed to set meta.source", slog.String(logging.FieldError, err.Error()))
	}
}

func (m MetaSourceSetter) do(responseBody *[]byte) error {
	resource := make(map[string]interface{})
	if err := json.Unmarshal(*responseBody, &resource); err != nil {
		return err
	}
	meta, ok := resource["meta"].(map[string]interface{})
	if !ok {
		meta = make(map[string]interface{})
		resource["meta"] = meta
	}
	meta["source"] = m.URI
	newBody, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	*responseBody = newBody
	return nil
}

var _ HttpResponseTransformer = &ResponseBodyRewriter{}

// ResponseBodyRewriter is a transformer that rewrites the response body.
// It performs a byte slice replace on the response body.
type ResponseBodyRewriter struct {
	Old []byte
	New []byte
}

func (u ResponseBodyRewriter) Transform(_ *int, responseBody *[]byte, _ map[string][]string) {
	*responseBody = bytes.ReplaceAll(*responseBody, u.Old, u.New)
}

var _ HttpResponseTransformer = &ResponseHeaderRewriter{}

// ResponseHeaderRewriter is a transformer that rewrites the response headers.
// It performs a string replace on the values of all headers.
type ResponseHeaderRewriter struct {
	Old string
	New string
}

func (h ResponseHeaderRewriter) Transform(_ *int, _ *[]byte, responseHeaders map[string][]string) {
	for headerName, headerValues := range responseHeaders {
		for headerValueIdx, headerValue := range headerValues {
			responseHeaders[headerName][headerValueIdx] = strings.ReplaceAll(headerValue, h.Old, h.New)
		}
	}
}

var _ HttpResponseTransformer = &ResponseHeaderSetter{}

// ResponseHeaderSetter is a transformer that sets HTTP response headers.
type ResponseHeaderSetter http.Header

func (r ResponseHeaderSetter) Transform(_ *int, _ *[]byte, responseHeaders map[string][]string) {
	if r != nil {
		for headerName, headerValues := range r {
			responseHeaders[headerName] = headerValues
		}
	}
}

var _ HttpResponseTransformer = &StripResponseHeaders{}

// StripResponseHeaders is a transformer that removes all HTTP response headers
// that are not in the allowlist.

var allowedHeaders = []string{
	// FHIR-specific headers
	"content-type",
	"content-length",
	"location",
	"etag",
	"last-modified",
	// CORS headers
	"access-control-allow-origin",
	"access-control-allow-methods",
	"access-control-allow-headers",
	// Security headers
	"x-content-type-options",
	"x-frame-options",
	"content-security-policy",
	// Caching headers
	"cache-control",
	"expires",
}

type StripResponseHeaders struct{}

func (h StripResponseHeaders) Transform(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string) {
	for headerName := range responseHeaders {
		// Header name cases can be inconsistent, so we normalize to lower case for comparison
		if !slices.Contains(allowedHeaders, strings.ToLower(headerName)) {
			delete(responseHeaders, headerName)
		}
	}
}
