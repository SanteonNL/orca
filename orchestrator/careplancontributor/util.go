package careplancontributor

import (
	"bytes"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

var _ http.RoundTripper = internalDispatchHTTPRoundTripper{}

// internalDispatchHTTPRoundTripper is an http.RoundTripper that forwards the request to an in-process HTTP handler.
type internalDispatchHTTPRoundTripper struct {
	profile        profile.Provider
	handler        http.Handler
	requestVisitor func(*http.Request)
}

type memoryResponseWriter struct {
	headers http.Header
	body    *bytes.Buffer
	status  int
}

func (d *memoryResponseWriter) Header() http.Header {
	return d.headers
}

func (d *memoryResponseWriter) Write(bytes []byte) (int, error) {
	return d.body.Write(bytes)
}

func (d *memoryResponseWriter) WriteHeader(statusCode int) {
	d.status = statusCode
}

func (i internalDispatchHTTPRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	responseWriter := &memoryResponseWriter{
		headers: make(http.Header),
		body:    new(bytes.Buffer),
	}
	identities, err := i.profile.Identities(request.Context())
	if err != nil {
		return nil, err
	}
	if len(identities) != 1 {
		return nil, errors.New("expected exactly one identity")
	}

	ctx := auth.WithPrincipal(request.Context(), auth.Principal{Organization: identities[0]})
	request = request.WithContext(ctx)
	if i.requestVisitor != nil {
		i.requestVisitor(request)
	}
	i.handler.ServeHTTP(responseWriter, request)
	log.Ctx(ctx).Info().Msgf("InternalDispatch HTTP Respone.status: %d", responseWriter.status)
	log.Ctx(ctx).Info().Msgf("InternalDispatch HTTP Respone.headers: %v", responseWriter.headers)
	return &http.Response{
		StatusCode: responseWriter.status,
		Header:     responseWriter.headers,
		Body:       io.NopCloser(bytes.NewReader(responseWriter.body.Bytes())),
		Request:    request,
	}, nil
}
