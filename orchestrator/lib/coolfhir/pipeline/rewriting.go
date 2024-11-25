package pipeline

import (
	"bytes"
	"net/http"
	"strings"
)

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
